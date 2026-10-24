package processor

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/client"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
)

func recoverMainServer(conn *net.Conn) {
	if err := recover(); err != nil {
		log.Errorf("mainServer panic: %v\n%s", err, debug.Stack())
		if conn != nil && *conn != nil {
			_ = (*conn).Close()
		}
	}
}

func recoverDataServer(conn *net.Conn) {
	if err := recover(); err != nil {
		log.Errorf("dataServer panic: %v\n%s", err, debug.Stack())
		if conn != nil && *conn != nil {
			_ = (*conn).Close()
		}
	}
}

func MainServer() {
	ln, err := net.Listen("tcp", ":"+common.GetOptions().MainServerPort)
	if err != nil {
		log.Fatalf("mainServer: %s", err)
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Errorf("mainServer accept: %s", err)
			return
		}
		conn = &controlConn{Conn: conn}
		go ping(&conn)
		go mainHandler(&conn)
	}
}

func mainHandler(conn *net.Conn) {
	id := common.RandomString(common.MaxLengthIDLog)
	defer recoverMainServer(conn)
	var curClient client.Client
	defer func() {
		_ = (*conn).Close()
		state := curClient.Snapshot()
		if state.Profile != nil {
			client.DelAuthorizedClient(state.Profile.Email, &curClient)
		}
		curClient.RemoveClient()
		if state.Pid != "" && len(client.GetClientsList(state.Pid)) == 0 {
			for _, p := range client.GetContainedProfileList(state.Pid) {
				for _, c := range client.GetAuthorizedClientList(p.Email) {
					sendMessage(c.Snapshot().Conn, TMessStatus, common.CleanPid(state.Pid), "0")
				}
			}
			client.DelContainedProfiles(state.Pid)
		}
	}()
	_ = (*conn).SetReadDeadline(time.Now().Add(common.WaitConnection * time.Second))
	reader := bufio.NewReader(*conn)
	for {
		message, err := readMessage(reader)
		if err != nil {
			return
		}
		if message.TMessage < 0 || message.TMessage >= len(Processing) {
			log.Infof("%s неизвестное сообщение: %d", id, message.TMessage)
			continue
		}
		if curClient.Snapshot().Pid == "" && message.TMessage != TMessAuth && message.TMessage != TMessVersion && message.TMessage != TMessPing {
			return
		}
		if handler := Processing[message.TMessage].Processing; handler != nil {
			ok := handler(message, conn, &curClient, id)
			if message.TMessage == TMessAuth {
				if !ok {
					return
				}
				_ = (*conn).SetReadDeadline(time.Time{})
			}
		}
	}
}

func DataServer() {
	ln, err := net.Listen("tcp", ":"+common.GetOptions().DataServerPort)
	if err != nil {
		log.Fatalf("dataServer: %s", err)
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Errorf("dataServer accept: %s", err)
			return
		}
		go dataHandler(&conn)
	}
}

type deadlineWriter struct{ net.Conn }

func (writer deadlineWriter) Write(p []byte) (int, error) {
	if err := writer.SetWriteDeadline(time.Now().Add(common.WriteTimeout * time.Second)); err != nil {
		return 0, err
	}
	return writer.Conn.Write(p)
}

func dataHandler(conn *net.Conn) {
	defer recoverDataServer(conn)
	defer (*conn).Close()
	_ = (*conn).SetReadDeadline(time.Now().Add(common.WaitConnection * time.Second))
	reader := bufio.NewReader(*conn)
	line, err := reader.ReadSlice('\n')
	if err != nil || len(line) <= 1 || len(line) > common.CodeLength+1 {
		return
	}
	code := string(line[:len(line)-1])
	value, exists := channels.Load(code)
	if !exists {
		return
	}
	peers := value.(*dConn)
	peers.mutex.Lock()
	if peers.closed || peers.pointer[1] != nil {
		peers.mutex.Unlock()
		return
	}
	index := 0
	if peers.pointer[0] != nil {
		index = 1
	}
	peers.pointer[index] = conn
	if index == 1 {
		close(peers.ready)
	}
	peers.mutex.Unlock()
	defer disconnectPair(code, peers)
	_ = (*conn).SetReadDeadline(time.Time{})

	config := common.GetOptions()
	if index == 0 {
		if config.Mode == common.ModeRegular {
			if peers.client == nil || peers.server == nil || !sendMessage(peers.client.Snapshot().Conn, TMessConnect, "", "", code, "simple", "client", peers.server.Snapshot().Pid, peers.address) {
				return
			}
		} else {
			sendMessageToMaster(TMessAgentNewConn, code)
		}
	}
	timer := time.NewTimer(common.WaitCount * common.WaitIdle * time.Millisecond)
	defer timer.Stop()
	select {
	case <-peers.ready:
	case <-timer.C:
		return
	}
	peers.mutex.RLock()
	other, closed := peers.pointer[1-index], peers.closed
	peers.mutex.RUnlock()
	if closed || other == nil {
		return
	}
	// Keep using reader: the code and VNC payload can arrive in one TCP packet.
	n, err := io.CopyBuffer(deadlineWriter{*other}, reader, make([]byte, config.SizeBuff))
	if err != nil {
		log.Debugf("dataServer copy: %s", err)
	}
	count := uint64(n) * 2
	common.AddCounter(count)
	if config.Mode == common.ModeNode {
		sendMessageToMaster(TMessAgentAddBytes, fmt.Sprint(count))
	}
}

func disconnectPeers(code string) {
	if value, exists := channels.Load(code); exists {
		disconnectPair(code, value.(*dConn))
	}
}

func disconnectPair(code string, pair *dConn) {
	if !channels.CompareAndDelete(code, pair) {
		return
	}
	pair.mutex.Lock()
	pair.closed = true
	connections := pair.pointer
	if connections[0] == nil || connections[1] == nil {
		close(pair.ready)
	}
	pair.mutex.Unlock()
	for _, conn := range connections {
		if conn != nil {
			_ = (*conn).Close()
		}
	}
	switch common.GetOptions().Mode {
	case common.ModeMaster:
		sendMessageToNodes(TMessAgentDelCode, code)
	case common.ModeNode:
		sendMessageToMaster(TMessAgentDelCode, code)
	}
}

func connectPeers(code string, receiving *client.Client, sending *client.Client, address string) {
	connection := &dConn{client: receiving, server: sending, address: address, ready: make(chan struct{})}
	if _, loaded := channels.LoadOrStore(code, connection); loaded {
		return
	}
	go checkConnection(connection, code)
	if common.GetOptions().Mode == common.ModeMaster {
		sendMessageToNodes(TMessAgentAddCode, code)
	}
}

func checkConnection(connection *dConn, code string) {
	timer := time.NewTimer(common.WaitConnection * time.Second)
	defer timer.Stop()
	select {
	case <-connection.ready:
		return
	case <-timer.C:
	}
	connection.mutex.RLock()
	expired := !connection.closed && connection.node == nil && (connection.pointer[0] == nil || connection.pointer[1] == nil)
	unused := connection.pointer[0] == nil && connection.pointer[1] == nil
	connection.mutex.RUnlock()
	if !expired {
		return
	}
	if common.GetOptions().Mode == common.ModeNode && unused {
		// Other nodes received this code too; an unused copy must not end their session.
		connection.mutex.Lock()
		if connection.pointer[0] == nil && channels.CompareAndDelete(code, connection) {
			connection.closed = true
			close(connection.ready)
		}
		connection.mutex.Unlock()
		return
	}
	if connection.client != nil && connection.client.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		sendMessage(connection.client.Snapshot().Conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageTimeoutError))
	}
	disconnectPair(code, connection)
}
