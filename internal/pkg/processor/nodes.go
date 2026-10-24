package processor

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
)

func MasterServer() {
	ln, err := net.Listen("tcp", ":"+common.GetOptions().MasterPort)
	if err != nil {
		log.Fatalf("masterServer: %s", err)
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Errorf("masterServer accept: %s", err)
			return
		}
		conn = &controlConn{Conn: conn}
		go ping(&conn)
		go masterHandler(&conn)
	}
}

func masterHandler(conn *net.Conn) {
	id := common.RandomString(common.MaxLengthIDLog)
	var curNode Node
	defer recoverMainServer(conn)
	defer func() {
		_ = (*conn).Close()
		if curNode.Id == "" {
			return
		}
		nodes.Delete(curNode.Id)
		sendMessageToAllClients(TMessServers, fmt.Sprint(false), curNode.Ip)
		channels.Range(func(key, value interface{}) bool {
			pair := value.(*dConn)
			pair.mutex.RLock()
			owned := pair.node == &curNode
			pair.mutex.RUnlock()
			if owned {
				disconnectPair(key.(string), pair)
			}
			return true
		})
	}()
	_ = (*conn).SetReadDeadline(time.Now().Add(common.WaitConnection * time.Second))
	reader := bufio.NewReader(*conn)
	for {
		message, err := readMessage(reader)
		if err != nil {
			return
		}
		if message.TMessage < 0 || message.TMessage >= len(ProcessingAgent) {
			continue
		}
		if curNode.Id == "" && message.TMessage != TMessAgentAuth && message.TMessage != TMessAgentPing {
			return
		}
		if message.TMessage == TMessAgentAuth && curNode.Id != "" {
			return
		}
		if handler := ProcessingAgent[message.TMessage].Processing; handler != nil {
			// Preserve command order, including authorization and session creation.
			handler(message, conn, &curNode, id)
			if message.TMessage == TMessAgentAuth {
				if curNode.Id == "" {
					return
				}
				_ = (*conn).SetReadDeadline(time.Time{})
			}
		}
	}
}

func NodeClient() {
	for {
		config := common.GetOptions()
		raw, err := net.DialTimeout("tcp", net.JoinHostPort(config.MasterServer, config.MasterPort), common.WaitConnection*time.Second)
		if err != nil {
			log.Errorf("nodeClient connect: %s", err)
			time.Sleep(common.WaitIdleAgent * time.Second)
			continue
		}
		var conn net.Conn = &controlConn{Conn: raw}
		masterMutex.Lock()
		master = &conn
		masterMutex.Unlock()
		hostname := config.Hostname
		if hostname == "" {
			hostname, err = os.Hostname()
			if err != nil {
				hostname = common.RandomString(common.MaxLengthIDNode)
			}
		}
		locationMutex.RLock()
		position := coordinates
		locationMutex.RUnlock()
		_ = conn.SetReadDeadline(time.Now().Add(common.WaitConnection * time.Second))
		if sendMessage(&conn, TMessAgentAuth, hostname, config.MasterPassword, common.WhitelabelVersion, fmt.Sprintf("%f;%f", position[0], position[1])) {
			go ping(&conn)
			nodeReadLoop(&conn)
		}
		_ = conn.Close()
		masterMutex.Lock()
		if master == &conn {
			master = nil
		}
		masterMutex.Unlock()
		time.Sleep(common.WaitIdleAgent * time.Second)
	}
}

func nodeReadLoop(conn *net.Conn) {
	reader := bufio.NewReader(*conn)
	authorized := false
	for {
		message, err := readMessage(reader)
		if err != nil {
			return
		}
		if message.TMessage < 0 || message.TMessage >= len(ProcessingAgent) {
			continue
		}
		if message.TMessage == TMessAgentAuth {
			if authorized || len(message.Messages) != 1 || message.Messages[0] == "" {
				return
			}
			authorized = true
			_ = (*conn).SetReadDeadline(time.Time{})
			continue
		}
		if !authorized && message.TMessage != TMessAgentPing {
			return
		}
		if handler := ProcessingAgent[message.TMessage].Processing; handler != nil {
			handler(message, conn, nil, common.RandomString(common.MaxLengthIDLog))
		}
	}
}

func processAgentAuth(message Message, conn *net.Conn, curNode *Node, id string) {
	config := common.GetOptions()
	if config.Mode != common.ModeMaster || curNode == nil || len(message.Messages) < 3 || message.Messages[2] != common.WhitelabelVersion || message.Messages[1] != config.MasterPassword {
		_ = (*conn).Close()
		return
	}
	curNode.Conn, curNode.Name = conn, message.Messages[0]
	curNode.Ip, _, _ = net.SplitHostPort((*conn).RemoteAddr().String())
	if len(message.Messages) > 3 {
		parts := strings.Split(message.Messages[3], ";")
		if len(parts) == 2 {
			curNode.coordinates[0], _ = strconv.ParseFloat(parts[0], 64)
			curNode.coordinates[1], _ = strconv.ParseFloat(parts[1], 64)
		}
	} else {
		curNode.coordinates = common.GetCoordinatesByYandex(curNode.Ip)
	}
	curNode.Id = common.RandomString(common.MaxLengthIDNode)
	if !sendMessage(conn, TMessAgentAuth, curNode.Id) {
		curNode.Id = ""
		return
	}
	nodes.Store(curNode.Id, curNode)
	sendMessageToAllClients(TMessServers, fmt.Sprint(true), curNode.Ip)
}

func processAgentAddCode(message Message, _ *net.Conn, _ *Node, _ string) {
	if common.GetOptions().Mode == common.ModeNode && len(message.Messages) == 1 && len(message.Messages[0]) > 0 && len(message.Messages[0]) <= common.CodeLength {
		connectPeers(message.Messages[0], nil, nil, "")
	}
}

func processAgentDelCode(message Message, _ *net.Conn, _ *Node, _ string) {
	if common.GetOptions().Mode != common.ModeRegular && len(message.Messages) == 1 {
		disconnectPeers(message.Messages[0])
	}
}

func processAgentAddBytes(message Message, _ *net.Conn, _ *Node, _ string) {
	if common.GetOptions().Mode != common.ModeMaster || len(message.Messages) != 1 {
		return
	}
	if count, err := strconv.ParseUint(message.Messages[0], 10, 64); err == nil {
		common.AddCounter(count)
	}
}

func sendMessageToNodes(kind int, messages ...string) {
	nodes.Range(func(_, value interface{}) bool {
		sendMessage(value.(*Node).Conn, kind, messages...)
		return true
	})
}

func sendMessageToMaster(kind int, messages ...string) {
	masterMutex.RLock()
	conn := master
	masterMutex.RUnlock()
	sendMessage(conn, kind, messages...)
}

func processAgentNewConn(message Message, _ *net.Conn, curNode *Node, _ string) {
	if common.GetOptions().Mode != common.ModeMaster || curNode == nil || len(message.Messages) != 1 {
		return
	}
	code := message.Messages[0]
	value, exists := channels.Load(code)
	if !exists {
		return
	}
	peers := value.(*dConn)
	peers.mutex.Lock()
	if peers.closed || peers.node != nil || peers.client == nil || peers.server == nil {
		peers.mutex.Unlock()
		return
	}
	peers.node = curNode
	peers.mutex.Unlock()
	sendMessage(peers.client.Snapshot().Conn, TMessConnect, "", "", code, "simple", "client", peers.server.Snapshot().Pid, peers.address)
}

func processAgentPing(_ Message, _ *net.Conn, _ *Node, _ string) {}
