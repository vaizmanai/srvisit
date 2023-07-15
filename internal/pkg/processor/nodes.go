package processor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func MasterServer() {
	log.Infof("masterServer запустился")

	ln, err := net.Listen("tcp", fmt.Sprintf(":%s", common.Options.MasterPort))
	if err != nil {
		log.Errorf("masterServer не смог занять порт")
		os.Exit(1)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Errorf("masterServer не смог занять сокет")
			break
		}

		go ping(&conn)
		go masterHandler(&conn)
	}

	_ = ln.Close()
	log.Infof("masterServer остановился")
}

func masterHandler(conn *net.Conn) {
	id := common.RandomString(common.MaxLengthIDLog)
	log.Infof("%s masterServer получил соединение", id)

	var curNode Node

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
			log.Infof("%s masterServer удаляем мусор", id)
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
		if len(ProcessingAgent) > message.TMessage {
			if ProcessingAgent[message.TMessage].Processing != nil {
				go ProcessingAgent[message.TMessage].Processing(message, conn, &curNode, id) //от одного агента может много приходить сообщений, не тормозим их
			} else {
				log.Infof("%s нет обработчика для сообщения %d", id, message.TMessage)
				time.Sleep(time.Millisecond * common.WaitIdle)
			}
		} else {
			log.Infof("%s неизвестное сообщение: %d", id, message.TMessage)
			time.Sleep(time.Millisecond * common.WaitIdle)
		}

	}
	(*conn).Close()

	//если есть id значит скорее всего есть в карте
	if len(curNode.Id) != 0 {
		nodes.Delete(curNode.Id)
		sendMessageToAllClients(TMessServers, fmt.Sprint(false), curNode.Ip)
	}

	//удалим все сессии связанные с этим агентом
	channels.Range(func(key interface{}, value interface{}) bool {
		dConn := value.(*dConn)
		if dConn.node == &curNode {
			channels.Delete(key)
		}
		return true
	})

	log.Infof("%s masterServer потерял соединение с агентом", id)
}

func NodeClient() {

	log.Infof("nodeClient запустился")

	for {
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", common.Options.MasterServer, common.Options.MasterPort))
		if err != nil {
			log.Errorf("nodeClient не смог подключиться: %s", err.Error())
			time.Sleep(time.Second * common.WaitIdleAgent)
			continue
		}

		master = &conn

		hostname, err := os.Hostname()
		if err != nil {
			hostname = common.RandomString(common.MaxLengthIDNode)
		}
		if len(common.Options.Hostname) > 0 {
			hostname = common.Options.Hostname
		}
		sendMessage(&conn, TMessAgentAuth, hostname, common.Options.MasterPassword, common.WhitelabelVersion, fmt.Sprintf("%f;%f", coordinates[0], coordinates[1]))

		go ping(&conn)

		reader := bufio.NewReader(conn)
		for {
			buff, err := reader.ReadBytes('}')

			if err != nil {
				log.Errorf("nodeClient ошибка чтения буфера: %s", err.Error())
				break
			}

			log.Debugf("buff (%d): %s", len(buff), buff)

			//удаляем мусор
			if buff[0] != '{' {
				log.Infof("nodeClient удаляем мусор")
				if bytes.Index(buff, []byte("{")) >= 0 {
					log.Debugf("buff (%d): %s", len(buff), buff)
					buff = buff[bytes.Index(buff, []byte("{")):]
				} else {
					continue
				}
			}

			var message Message
			err = json.Unmarshal(buff, &message)
			if err != nil {
				log.Errorf("nodeClient ошибка разбора json: %s", err.Error())
				time.Sleep(time.Millisecond * common.WaitIdle)
				continue
			}

			log.Debugf("%+v", message)

			//обрабатываем полученное сообщение
			if len(ProcessingAgent) > message.TMessage {
				if ProcessingAgent[message.TMessage].Processing != nil {
					go ProcessingAgent[message.TMessage].Processing(message, &conn, nil, common.RandomString(common.MaxLengthIDLog))
				} else {
					log.Infof("nodeClient нет обработчика для сообщения")
					time.Sleep(time.Millisecond * common.WaitIdle)
				}
			} else {
				log.Infof("nodeClient неизвестное сообщение: %d", message.TMessage)
				time.Sleep(time.Millisecond * common.WaitIdle)
			}

		}
		conn.Close()
	}

	//log.Infof("nodeClient остановился") //недостижимо???
}

func processAgentAuth(message Message, conn *net.Conn, curNode *Node, id string) {
	log.Infof("%s пришла авторизация агента", id)

	if common.Options.Mode == common.ModeRegular {
		log.Errorf("%s режим не поддерживающий агентов", id)
		(*conn).Close()
		return
	}

	if common.Options.Mode == common.ModeNode {
		log.Infof("%s пришел ответ на авторизацию", id)
		return
	}

	time.Sleep(time.Millisecond * common.WaitIdle)

	if len(message.Messages) < 3 {
		log.Errorf("%s не правильное кол-во полей", id)
		(*conn).Close()
		return
	}

	if message.Messages[2] != common.WhitelabelVersion {
		log.Errorf("%s не совместимая версия", id)
		(*conn).Close()
		return
	}

	if message.Messages[1] != common.Options.MasterPassword {
		log.Errorf("%s не правильный пароль", id)
		(*conn).Close()
		return
	}

	if len(message.Messages) > 3 {
		c := strings.Split(message.Messages[3], ";")
		if len(c) == 2 {
			c0, err := strconv.ParseFloat(c[0], 64)
			if err == nil {
				curNode.coordinates[0] = c0
			}
			c1, err := strconv.ParseFloat(c[1], 64)
			if err == nil {
				curNode.coordinates[1] = c1
			}
		}
	} else {
		//получим координаты по ip
		go func() {
			curNode.coordinates = common.GetCoordinatesByYandex(curNode.Ip)
		}()
	}

	curNode.Conn = conn
	curNode.Name = message.Messages[0]
	curNode.Id = common.RandomString(common.MaxLengthIDNode)

	h, _, err := net.SplitHostPort((*conn).RemoteAddr().String())
	if err == nil {
		curNode.Ip = h
	}

	if sendMessage(conn, TMessAgentAuth, curNode.Id) {
		nodes.Store(curNode.Id, curNode)
		log.Infof("%s авторизация агента успешна", id)
	}

	sendMessageToAllClients(TMessServers, fmt.Sprint(true), curNode.Ip)
}

func processAgentAddCode(message Message, conn *net.Conn, curNode *Node, id string) {
	if common.Options.Mode != common.ModeNode {
		log.Errorf("%s режим не поддерживающий агентов", id)
		return
	}

	log.Infof("%s пришла информация о создании сессии", id)

	if len(message.Messages) != 1 {
		log.Errorf("%s не правильное кол-во полей", id)
		return
	}

	connectPeers(message.Messages[0], nil, nil, "")
}

func processAgentDelCode(message Message, conn *net.Conn, curNode *Node, id string) {
	if common.Options.Mode == common.ModeRegular {
		log.Errorf("%s режим не поддерживающий агентов", id)
		return
	}

	log.Infof("%s пришла информация об удалении сессии", id)

	if len(message.Messages) != 1 {
		log.Errorf("%s не правильное кол-во полей", id)
		return
	}

	disconnectPeers(message.Messages[0])
}

func processAgentAddBytes(message Message, conn *net.Conn, curNode *Node, id string) {
	if common.Options.Mode != common.ModeMaster {
		log.Errorf("%s режим не поддерживающий агентов", id)
		return
	}

	log.Infof("%s пришла информация статистики", id)

	if len(message.Messages) != 1 {
		log.Errorf("%s не правильное кол-во полей", id)
		return
	}

	count, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		common.AddCounter(uint64(count))
	}
}

func sendMessageToNodes(TMessage int, Messages ...string) {
	nodes.Range(func(key interface{}, value interface{}) bool {
		node := value.(*Node)
		return sendMessage(node.Conn, TMessage, Messages...)
	})
}

func sendMessageToMaster(TMessage int, Messages ...string) {
	sendMessage(master, TMessage, Messages...)
}

func processAgentNewConn(message Message, conn *net.Conn, curNode *Node, id string) {
	if common.Options.Mode != common.ModeMaster {
		log.Errorf("%s режим не поддерживающий агентов", id)
		return
	}

	log.Infof("%s пришла информация о том что агент получил соединение", id)

	if len(message.Messages) != 1 {
		log.Errorf("%s не правильное кол-во полей", id)
		return
	}

	code := message.Messages[0]
	value, exists := channels.Load(code)
	if exists {
		peers := value.(*dConn)
		peers.node = curNode
		//отправим запрос принимающей стороне
		if !sendMessage(peers.client.Conn, TMessConnect, "", "", code, "simple", "client", peers.server.Pid, peers.address) {
			log.Errorf("%s не смогли отправить запрос принимающей стороне", id)
		}
	}
}

func processAgentPing(_ Message, _ *net.Conn, _ *Node, _ string) {
	//log.Infof("%s пришел пинг", id)
}
