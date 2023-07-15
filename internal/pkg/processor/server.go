package processor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/client"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
	"net"
	"runtime/debug"
	"time"
)

func recoverMainServer(conn *net.Conn) {
	if recover() != nil {
		log.Errorf("поток mainServer поймал критическую ошибку")
		debug.PrintStack()

		if conn != nil {
			_ = (*conn).Close()
		}
	}
}

func recoverDataServer(conn *net.Conn) {
	if recover() != nil {
		log.Errorf("поток dataServer поймал критическую ошибку")
		debug.PrintStack()

		if conn != nil {
			_ = (*conn).Close()
		}
	}
}

func MainServer() {
	log.Infof("mainServer запустился")

	ln, err := net.Listen("tcp", ":"+common.Options.MainServerPort)
	if err != nil {
		log.Errorf("mainServer не смог занять порт")
		panic(err.Error())
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Errorf("mainServer не смог занять сокет")
			break
		}

		go ping(&conn)
		go mainHandler(&conn)
	}

	_ = ln.Close()
	log.Infof("mainServer остановился")
}

func mainHandler(conn *net.Conn) {
	id := common.RandomString(common.MaxLengthIDLog)
	log.Infof("%s mainServer получил соединение %s", id, (*conn).RemoteAddr())

	defer recoverMainServer(conn)

	var curClient client.Client
	reader := bufio.NewReader(*conn)

	for {
		buff, err := reader.ReadBytes('}')

		if err != nil {
			log.Errorf("%s ошибка чтения буфера", id)
			break
		}

		log.Debugf("%s buff (%d): %s", id, len(buff), buff)

		//удаляем мусор
		if buff[0] != '{' {
			log.Infof("%s mainServer удаляем мусор", id)
			if bytes.Index(buff, []byte("{")) >= 0 {
				buff = buff[bytes.Index(buff, []byte("{")):]
			} else {
				continue
			}
		}

		var message Message
		err = json.Unmarshal(buff, &message)
		if err != nil {
			log.Errorf("%s ошибка разбора json", id)
			time.Sleep(time.Millisecond * common.WaitIdle)
			continue
		}

		log.Debugf("%s %+v", id, message)

		//обрабатываем полученное сообщение
		if len(Processing) > message.TMessage {
			if Processing[message.TMessage].Processing != nil {
				Processing[message.TMessage].Processing(message, conn, &curClient, id)
			} else {
				log.Infof("%s нет обработчика для сообщения %d", id, message.TMessage)
				time.Sleep(time.Millisecond * common.WaitIdle)
			}
		} else {
			log.Infof("%s неизвестное сообщение: %d", id, message.TMessage)
			time.Sleep(time.Millisecond * common.WaitIdle)
		}

	}
	_ = (*conn).Close()

	//удалим связи с профилем
	if curClient.Profile != nil {
		client.DelAuthorizedClient(curClient.Profile.Email, &curClient)
		client.DelContainedProfile(curClient.Pid, curClient.Profile)
	}

	//пробежимся по профилям где мы есть и отправим новый статус
	for _, profile := range client.GetContainedProfileList(curClient.Pid) {
		//все кто авторизовался в этот профиль должен получить новый статус
		for _, c := range client.GetAuthorizedClientList(profile.Email) {
			sendMessage(c.Conn, TMessStatus, common.CleanPid(curClient.Pid), "0")
		}
	}

	//удалим себя из карты клиентов
	curClient.RemoveClient()

	log.Infof("%s mainServer потерял соединение с пиром %s", id, (*conn).RemoteAddr())
}

func DataServer() {
	log.Infof("dataServer запустился")

	ln, err := net.Listen("tcp", fmt.Sprintf(":%s", common.Options.DataServerPort))
	if err != nil {
		log.Fatalf("dataServer не смог занять порт")
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Errorf("dataServer не смог занять сокет")
			break
		}

		go dataHandler(&conn)
	}

	_ = ln.Close()
	log.Infof("dataServer остановился")
}

func dataHandler(conn *net.Conn) {
	id := common.RandomString(6)
	log.Infof("%s dataHandler получил соединение %s", id, (*conn).RemoteAddr())

	defer recoverDataServer(conn)

	for {
		code, err := bufio.NewReader(*conn).ReadString('\n')

		if err != nil {
			log.Errorf("%s ошибка чтения кода", id)
			break
		}

		code = code[:len(code)-1]
		value, exist := channels.Load(code)
		if exist == false {
			log.Errorf("%s не ожидаем такого кода", id)
			break
		}

		peers := value.(*dConn)
		peers.mutex.Lock()
		var numPeer int
		if peers.pointer[0] == nil {
			peers.pointer[0] = conn
			numPeer = 1

			if common.Options.Mode == common.ModeRegular {
				//отправим запрос принимающей стороне
				if !sendMessage(peers.client.Conn, TMessConnect, "", "", code, "simple", "client", peers.server.Pid, peers.address) {
					log.Errorf("%s не смогли отправить запрос принимающей стороне", id)
				}
			} else { //options.mode == ModeNode
				sendMessageToMaster(TMessAgentNewConn, code) //оповестим мастер о том что мы дождались транслятор
			}

		} else if peers.pointer[1] == nil {
			peers.pointer[1] = conn
			numPeer = 0
		}
		peers.mutex.Unlock()

		var cWait = 0
		for peers.pointer[numPeer] == nil && cWait < common.WaitCount {
			log.Infof("%s ожидаем пира для %s", id, code)
			time.Sleep(time.Millisecond * common.WaitIdle)
			cWait++
		}

		if peers.pointer[numPeer] == nil {
			log.Errorf("%s превышено время ожидания", id)
			disconnectPeers(code)
			break
		}

		log.Infof("%s пир существует для %s", id, code)
		time.Sleep(time.Millisecond * common.WaitAfterConnect)

		var z []byte
		z = make([]byte, common.Options.SizeBuff)

		var countBytes uint64
		var n1, n2 int
		var err1, err2 error

		for {
			n1, err1 = (*conn).Read(z)

			if peers.pointer[numPeer] == nil {
				log.Infof("%s потеряли пир", id)
				time.Sleep(time.Millisecond * common.WaitAfterConnect)
				break
			}

			err := (*peers.pointer[numPeer]).SetWriteDeadline(time.Now().Add(time.Second * common.WriteTimeout))
			if err != nil {
				log.Infof("%s не получилось установит таймаут", id)
				break
			}
			n2, err2 = (*peers.pointer[numPeer]).Write(z[:n1])

			countBytes = countBytes + uint64(n1+n2)

			if err1 != nil || err2 != nil || n1 == 0 || n2 == 0 {
				log.Infof("%s соединение закрылось: %d %d", id, n1, n2)
				log.Infof("%s err1: %v", id, err1)
				log.Infof("%s err2: %v", id, err2)
				time.Sleep(time.Millisecond * common.WaitAfterConnect)
				if peers.pointer[numPeer] != nil {
					_ = (*peers.pointer[numPeer]).Close()
				}
				break
			}
		}

		common.AddCounter(countBytes)
		if common.Options.Mode == common.ModeNode {
			sendMessageToMaster(TMessAgentAddBytes, fmt.Sprint(countBytes))
		}

		log.Infof("%s поток завершается", id)
		disconnectPeers(code)
		break
	}
	_ = (*conn).Close()
	log.Infof("%s dataHandler потерял соединение", id)
}

func disconnectPeers(code string) {
	value, exists := channels.Load(code)
	if exists {
		channels.Delete(code)
		pair := value.(*dConn)

		if common.Options.Mode != common.ModeMaster {
			if pair.pointer[0] != nil {
				_ = (*pair.pointer[0]).Close()
			}
			if pair.pointer[1] != nil {
				_ = (*pair.pointer[1]).Close()
			}
		}
		if common.Options.Mode == common.ModeMaster {
			sendMessageToNodes(TMessAgentDelCode, code)
		}
		if common.Options.Mode == common.ModeNode {
			sendMessageToMaster(TMessAgentDelCode, code)
		}

		pair.client = nil
		pair.server = nil
	}
}

func connectPeers(code string, client *client.Client, server *client.Client, address string) {
	var newConnection dConn
	channels.Store(code, &newConnection)
	newConnection.client = client
	newConnection.server = server
	newConnection.address = address

	go checkConnection(&newConnection, code) //может случиться так, что код сохранили, а никто не подключился

	if common.Options.Mode == common.ModeMaster {
		sendMessageToNodes(TMessAgentAddCode, code)
	}
}

func checkConnection(connection *dConn, code string) {
	time.Sleep(time.Second * common.WaitConnection)

	if common.Options.Mode != common.ModeNode {
		if connection.node == nil && connection.pointer[0] == nil && connection.pointer[1] == nil {
			log.Errorf("таймаут ожидания соединений для %s", code)
			if connection.client != nil {
				if connection.client.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
					sendMessage(connection.client.Conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageTimeoutError))
				}
			}
			disconnectPeers(code)
		}
	} else {
		if (connection.pointer[0] != nil && connection.pointer[1] == nil) || (connection.pointer[0] == nil && connection.pointer[1] != nil) {
			log.Errorf("таймаут ожидания соединений для %s", code)
			disconnectPeers(code)
		}
	}
}
