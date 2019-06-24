package main

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
)

func masterServer() {
	logAdd(MESS_INFO, "masterServer запустился")

	ln, err := net.Listen("tcp", ":"+options.MasterPort)
	if err != nil {
		logAdd(MESS_ERROR, "masterServer не смог занять порт")
		os.Exit(1)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			logAdd(MESS_ERROR, "masterServer не смог занять сокет")
			break
		}

		go ping(&conn)
		go masterHandler(&conn)
	}

	ln.Close()
	logAdd(MESS_INFO, "masterServer остановился")
}

func masterHandler(conn *net.Conn) {
	id := randomString(MAX_LEN_ID_LOG)
	logAdd(MESS_INFO, id+" masterServer получил соединение")

	var curNode Node

	reader := bufio.NewReader(*conn)

	for {
		buff, err := reader.ReadBytes('}')

		if err != nil {
			logAdd(MESS_ERROR, id+" ошибка чтения буфера")
			break
		}

		logAdd(MESS_DETAIL, id+fmt.Sprint(" buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))

		//удаляем мусор
		if buff[0] != '{' {
			logAdd(MESS_INFO, id+" masterServer удаляем мусор")
			if bytes.Index(buff, []byte("{")) >= 0 {
				buff = buff[bytes.Index(buff, []byte("{")):]
			} else {
				continue
			}
		}

		var message Message
		err = json.Unmarshal(buff, &message)
		if err != nil {
			logAdd(MESS_ERROR, id+" ошибка разбора json")
			time.Sleep(time.Millisecond * WAIT_IDLE)
			continue
		}

		logAdd(MESS_DETAIL, id+" "+fmt.Sprint(message))

		//обрабатываем полученное сообщение
		if len(processingAgent) > message.TMessage {
			if processingAgent[message.TMessage].Processing != nil {
				go processingAgent[message.TMessage].Processing(message, conn, &curNode, id) //от одного агента может много приходить сообщений, не тормозим их
			} else {
				logAdd(MESS_INFO, id+" нет обработчика для сообщения "+fmt.Sprint(message.TMessage))
				time.Sleep(time.Millisecond * WAIT_IDLE)
			}
		} else {
			logAdd(MESS_INFO, id+" неизвестное сообщение: "+fmt.Sprint(message.TMessage))
			time.Sleep(time.Millisecond * WAIT_IDLE)
		}

	}
	(*conn).Close()

	//если есть id значит скорее всего есть в карте
	if len(curNode.Id) != 0 {
		nodes.Delete(curNode.Id)
		sendMessageToClients(TMESS_SERVERS, fmt.Sprint(false), curNode.Ip)
	}

	//удалим все сессии связанные с этим агентом
	channels.Range(func(key interface{}, value interface{}) bool {
		dConn := value.(*dConn)
		if dConn.node == &curNode {
			channels.Delete(key)
		}
		return true
	})

	logAdd(MESS_INFO, id+" masterServer потерял соединение с агентом")
}

func nodeClient() {

	logAdd(MESS_INFO, "nodeClient запустился")

	for {
		conn, err := net.Dial("tcp", options.MasterServer+":"+options.MasterPort)
		if err != nil {
			logAdd(MESS_ERROR, "nodeClient не смог подключиться: "+fmt.Sprint(err))
			time.Sleep(time.Second * WAIT_IDLE_AGENT)
			continue
		}

		master = &conn

		hostname, err := os.Hostname()
		if err != nil {
			hostname = randomString(MAX_LEN_ID_NODE)
		}
		if len(options.Hostname) > 0 {
			hostname = options.Hostname
		}
		sendMessage(&conn, TMESS_AGENT_AUTH, hostname, options.MasterPassword, REVISIT_VERSION, fmt.Sprint(coordinates[0], ";", coordinates[1]))

		go ping(&conn)

		reader := bufio.NewReader(conn)
		for {
			buff, err := reader.ReadBytes('}')

			if err != nil {
				logAdd(MESS_ERROR, "nodeClient ошибка чтения буфера: "+fmt.Sprint(err))
				break
			}

			logAdd(MESS_DETAIL, fmt.Sprint("buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))

			//удаляем мусор
			if buff[0] != '{' {
				logAdd(MESS_INFO, "nodeClient удаляем мусор")
				if bytes.Index(buff, []byte("{")) >= 0 {
					logAdd(MESS_DETAIL, fmt.Sprint("buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))
					buff = buff[bytes.Index(buff, []byte("{")):]
				} else {
					continue
				}
			}

			var message Message
			err = json.Unmarshal(buff, &message)
			if err != nil {
				logAdd(MESS_ERROR, "nodeClient ошибка разбора json: "+fmt.Sprint(err))
				time.Sleep(time.Millisecond * WAIT_IDLE)
				continue
			}

			logAdd(MESS_DETAIL, fmt.Sprint(message))

			//обрабатываем полученное сообщение
			if len(processingAgent) > message.TMessage {
				if processingAgent[message.TMessage].Processing != nil {
					go processingAgent[message.TMessage].Processing(message, &conn, nil, randomString(MAX_LEN_ID_LOG))
				} else {
					logAdd(MESS_INFO, "nodeClient нет обработчика для сообщения")
					time.Sleep(time.Millisecond * WAIT_IDLE)
				}
			} else {
				logAdd(MESS_INFO, "nodeClient неизвестное сообщение: "+fmt.Sprint(message.TMessage))
				time.Sleep(time.Millisecond * WAIT_IDLE)
			}

		}
		conn.Close()
	}

	//logAdd(MESS_INFO, "nodeClient остановился") //недостижимо???
}

func processAgentAuth(message Message, conn *net.Conn, curNode *Node, id string) {
	logAdd(MESS_INFO, id+" пришла авторизация агента")

	if options.mode == REGULAR {
		logAdd(MESS_ERROR, id+" режим не поддерживающий агентов")
		(*conn).Close()
		return
	}

	if options.mode == NODE {
		logAdd(MESS_INFO, id+" пришел ответ на авторизацию")
		return
	}

	time.Sleep(time.Millisecond * WAIT_IDLE)

	if len(message.Messages) < 3 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		(*conn).Close()
		return
	}

	if message.Messages[2] != REVISIT_VERSION {
		logAdd(MESS_ERROR, id+" не совместимая версия")
		(*conn).Close()
		return
	}

	if message.Messages[1] != options.MasterPassword {
		logAdd(MESS_ERROR, id+" не правильный пароль")
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
	curNode.Id = randomString(MAX_LEN_ID_NODE)

	h, _, err := net.SplitHostPort((*conn).RemoteAddr().String())
	if err == nil {
		curNode.Ip = h
	}

	if sendMessage(conn, TMESS_AGENT_AUTH, curNode.Id) {
		nodes.Store(curNode.Id, curNode)
		logAdd(MESS_INFO, id+" авторизация агента успешна")
	}

	sendMessageToClients(TMESS_SERVERS, fmt.Sprint(true), curNode.Ip)
}

func processAgentAddCode(message Message, conn *net.Conn, curNode *Node, id string) {
	if options.mode != NODE {
		logAdd(MESS_ERROR, id+" режим не поддерживающий агентов")
		return
	}

	logAdd(MESS_INFO, id+" пришла информация о создании сессии")

	if len(message.Messages) != 1 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	connectPeers(message.Messages[0], nil, nil, "")
}

func processAgentDelCode(message Message, conn *net.Conn, curNode *Node, id string) {
	if options.mode == REGULAR {
		logAdd(MESS_ERROR, id+" режим не поддерживающий агентов")
		return
	}

	logAdd(MESS_INFO, id+" пришла информация об удалении сессии")

	if len(message.Messages) != 1 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	disconnectPeers(message.Messages[0])
}

func processAgentAddBytes(message Message, conn *net.Conn, curNode *Node, id string) {
	if options.mode != MASTER {
		logAdd(MESS_ERROR, id+" режим не поддерживающий агентов")
		return
	}

	logAdd(MESS_INFO, id+" пришла информация статистики")

	if len(message.Messages) != 1 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	bytes, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		addCounter(uint64(bytes))
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
	if options.mode != MASTER {
		logAdd(MESS_ERROR, id+" режим не поддерживающий агентов")
		return
	}

	logAdd(MESS_INFO, id+" пришла информация о том что агент получил соединение")

	if len(message.Messages) != 1 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	code := message.Messages[0]
	value, exists := channels.Load(code)
	if exists {
		peers := value.(*dConn)
		peers.node = curNode
		//отправим запрос принимающей стороне
		if !sendMessage(peers.client.Conn, TMESS_CONNECT, "", "", code, "simple", "client", peers.server.Pid, peers.address) {
			logAdd(MESS_ERROR, id+" не смогли отправить запрос принимающей стороне")
		}
	}
}

func processAgentPing(message Message, conn *net.Conn, curNode *Node, id string) {
	//logAdd(MESS_INFO, id + " пришел пинг")
}
