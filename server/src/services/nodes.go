package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	"../common"
)

func MasterServer() {
	common.LogAdd(common.MESS_INFO, "masterServer запустился")

	ln, err := net.Listen("tcp", ":"+common.Options.MasterPort)
	if err != nil {
		common.LogAdd(common.MESS_ERROR, "masterServer не смог занять порт")
		os.Exit(1)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			common.LogAdd(common.MESS_ERROR, "masterServer не смог занять сокет")
			break
		}

		go common.Ping(&conn)
		go masterHandler(&conn)
	}

	ln.Close()
	common.LogAdd(common.MESS_INFO, "masterServer остановился")
}

func masterHandler(conn *net.Conn) {
	id := common.RandomString(common.MAX_LEN_ID_LOG)
	common.LogAdd(common.MESS_INFO, id+" masterServer получил соединение")

	var curNode common.Node

	reader := bufio.NewReader(*conn)

	for {
		buff, err := reader.ReadBytes('}')

		if err != nil {
			common.LogAdd(common.MESS_ERROR, id+" ошибка чтения буфера")
			break
		}

		common.LogAdd(common.MESS_DETAIL, id+fmt.Sprint(" buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))

		//удаляем мусор
		if buff[0] != '{' {
			common.LogAdd(common.MESS_INFO, id+" masterServer удаляем мусор")
			if bytes.Index(buff, []byte("{")) >= 0 {
				buff = buff[bytes.Index(buff, []byte("{")):]
			} else {
				continue
			}
		}

		var message common.Message
		err = json.Unmarshal(buff, &message)
		if err != nil {
			common.LogAdd(common.MESS_ERROR, id+" ошибка разбора json")
			time.Sleep(time.Millisecond * common.WAIT_IDLE)
			continue
		}

		common.LogAdd(common.MESS_DETAIL, id+" "+fmt.Sprint(message))

		//обрабатываем полученное сообщение
		if len(processingAgent) > message.TMessage {
			if processingAgent[message.TMessage].Processing != nil {
				go processingAgent[message.TMessage].Processing(message, conn, &curNode, id) //от одного агента может много приходить сообщений, не тормозим их
			} else {
				common.LogAdd(common.MESS_INFO, id+" нет обработчика для сообщения "+fmt.Sprint(message.TMessage))
				time.Sleep(time.Millisecond * common.WAIT_IDLE)
			}
		} else {
			common.LogAdd(common.MESS_INFO, id+" неизвестное сообщение: "+fmt.Sprint(message.TMessage))
			time.Sleep(time.Millisecond * common.WAIT_IDLE)
		}

	}
	(*conn).Close()

	//если есть id значит скорее всего есть в карте
	if len(curNode.Id) != 0 {
		nodes.Delete(curNode.Id)
		sendMessageToClients(common.TMESS_SERVERS, fmt.Sprint(false), curNode.Ip)
	}

	//удалим все сессии связанные с этим агентом
	channels.Range(func(key interface{}, value interface{}) bool {
		dConn := value.(*dConn)
		if dConn.node == &curNode {
			channels.Delete(key)
		}
		return true
	})

	common.LogAdd(common.MESS_INFO, id+" masterServer потерял соединение с агентом")
}

func NodeClient() {

	common.LogAdd(common.MESS_INFO, "nodeClient запустился")

	for {
		conn, err := net.Dial("tcp", common.Options.MasterServer+":"+common.Options.MasterPort)
		if err != nil {
			common.LogAdd(common.MESS_ERROR, "nodeClient не смог подключиться: "+fmt.Sprint(err))
			time.Sleep(time.Second * common.WAIT_IDLE_AGENT)
			continue
		}

		master = &conn

		hostname, err := os.Hostname()
		if err != nil {
			hostname = common.RandomString(common.MAX_LEN_ID_NODE)
		}
		if len(common.Options.Hostname) > 0 {
			hostname = common.Options.Hostname
		}
		common.SendMessage(&conn, common.TMESS_AGENT_AUTH, hostname, common.Options.MasterPassword, common.REVISIT_VERSION, fmt.Sprint(coordinates[0], ";", coordinates[1]))

		go common.Ping(&conn)

		reader := bufio.NewReader(conn)
		for {
			buff, err := reader.ReadBytes('}')

			if err != nil {
				common.LogAdd(common.MESS_ERROR, "nodeClient ошибка чтения буфера: "+fmt.Sprint(err))
				break
			}

			common.LogAdd(common.MESS_DETAIL, fmt.Sprint("buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))

			//удаляем мусор
			if buff[0] != '{' {
				common.LogAdd(common.MESS_INFO, "nodeClient удаляем мусор")
				if bytes.Index(buff, []byte("{")) >= 0 {
					common.LogAdd(common.MESS_DETAIL, fmt.Sprint("buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))
					buff = buff[bytes.Index(buff, []byte("{")):]
				} else {
					continue
				}
			}

			var message common.Message
			err = json.Unmarshal(buff, &message)
			if err != nil {
				common.LogAdd(common.MESS_ERROR, "nodeClient ошибка разбора json: "+fmt.Sprint(err))
				time.Sleep(time.Millisecond * common.WAIT_IDLE)
				continue
			}

			common.LogAdd(common.MESS_DETAIL, fmt.Sprint(message))

			//обрабатываем полученное сообщение
			if len(processingAgent) > message.TMessage {
				if processingAgent[message.TMessage].Processing != nil {
					go processingAgent[message.TMessage].Processing(message, &conn, nil, randomString(MAX_LEN_ID_LOG))
				} else {
					common.LogAdd(common.MESS_INFO, "nodeClient нет обработчика для сообщения")
					time.Sleep(time.Millisecond * common.WAIT_IDLE)
				}
			} else {
				common.LogAdd(common.MESS_INFO, "nodeClient неизвестное сообщение: "+fmt.Sprint(message.TMessage))
				time.Sleep(time.Millisecond * common.WAIT_IDLE)
			}

		}
		conn.Close()
	}

	//common.LogAdd(common.MESS_INFO, "nodeClient остановился") //недостижимо???
}

func ProcessAgentAuth(message common.Message, conn *net.Conn, curNode *common.Node, id string) {
	common.LogAdd(common.MESS_INFO, id+" пришла авторизация агента")

	if common.Options.Mode == common.REGULAR {
		common.LogAdd(common.MESS_ERROR, id+" режим не поддерживающий агентов")
		(*conn).Close()
		return
	}

	if common.Options.Mode == common.NODE {
		common.LogAdd(common.MESS_INFO, id+" пришел ответ на авторизацию")
		return
	}

	time.Sleep(time.Millisecond * common.WAIT_IDLE)

	if len(message.Messages) < 3 {
		common.LogAdd(common.MESS_ERROR, id+" не правильное кол-во полей")
		(*conn).Close()
		return
	}

	if message.Messages[2] != REVISIT_VERSION {
		common.LogAdd(common.MESS_ERROR, id+" не совместимая версия")
		(*conn).Close()
		return
	}

	if message.Messages[1] != common.Options.MasterPassword {
		common.LogAdd(common.MESS_ERROR, id+" не правильный пароль")
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
			curNode.coordinates = getCoordinatesByYandex(curNode.Ip)
		}()
	}

	curNode.Conn = conn
	curNode.Name = message.Messages[0]
	curNode.Id = randomString(common.MAX_LEN_ID_NODE)

	h, _, err := net.SplitHostPort((*conn).RemoteAddr().String())
	if err == nil {
		curNode.Ip = h
	}

	if common.SendMessage(conn, common.TMESS_AGENT_AUTH, curNode.Id) {
		nodes.Store(curNode.Id, curNode)
		common.LogAdd(common.MESS_INFO, id+" авторизация агента успешна")
	}

	common.sendMessageToClients(common.TMESS_SERVERS, fmt.Sprint(true), curNode.Ip)
}

func ProcessAgentAddCode(message common.Message, conn *net.Conn, curNode *common.Node, id string) {
	if common.Options.Mode != common.NODE {
		common.LogAdd(common.MESS_ERROR, id+" режим не поддерживающий агентов")
		return
	}

	common.LogAdd(common.MESS_INFO, id+" пришла информация о создании сессии")

	if len(message.Messages) != 1 {
		common.LogAdd(common.MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	connectPeers(message.Messages[0], nil, nil, "")
}

func ProcessAgentDelCode(message common.Message, conn *net.Conn, curNode *common.Node, id string) {
	if common.Options.Mode == common.REGULAR {
		common.LogAdd(common.MESS_ERROR, id+" режим не поддерживающий агентов")
		return
	}

	common.LogAdd(common.MESS_INFO, id+" пришла информация об удалении сессии")

	if len(message.Messages) != 1 {
		common.LogAdd(common.MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	disconnectPeers(message.Messages[0])
}

func ProcessAgentAddBytes(message common.Message, conn *net.Conn, curNode *common.Node, id string) {
	if common.Options.Mode != common.MASTER {
		common.LogAdd(common.MESS_ERROR, id+" режим не поддерживающий агентов")
		return
	}

	common.LogAdd(common.MESS_INFO, id+" пришла информация статистики")

	if len(message.Messages) != 1 {
		common.LogAdd(common.MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	bytes, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		common.AddCounter(uint64(bytes))
	}
}

func SendMessageToNodes(TMessage int, Messages ...string) {
	nodes.Range(func(key interface{}, value interface{}) bool {
		node := value.(*Node)
		return common.SendMessage(node.Conn, TMessage, Messages...)
	})
}

func sendMessageToMaster(TMessage int, Messages ...string) {
	common.SendMessage(master, TMessage, Messages...)
}

func ProcessAgentNewConn(message common.Message, conn *net.Conn, curNode *common.Node, id string) {
	if common.Options.Mode != common.MASTER {
		common.LogAdd(common.MESS_ERROR, id+" режим не поддерживающий агентов")
		return
	}

	common.LogAdd(common.MESS_INFO, id+" пришла информация о том что агент получил соединение")

	if len(message.Messages) != 1 {
		common.LogAdd(common.MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	code := message.Messages[0]
	value, exists := channels.Load(code)
	if exists {
		peers := value.(*dConn)
		peers.node = curNode
		//отправим запрос принимающей стороне
		if !common.SendMessage(peers.client.Conn, common.TMESS_CONNECT, "", "", code, "simple", "client", peers.server.Pid, peers.address) {
			common.LogAdd(common.MESS_ERROR, id+" не смогли отправить запрос принимающей стороне")
		}
	}
}

func ProcessAgentPing(message common.Message, conn *net.Conn, curNode *common.Node, id string) {
	//common.LogAdd(common.MESS_INFO, id + " пришел пинг")
}
