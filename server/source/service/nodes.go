package service

import (
	. "../common"
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

func MasterServer() {
	LogAdd(MessInfo, "masterServer запустился")

	ln, err := net.Listen("tcp", ":"+Options.MasterPort)
	if err != nil {
		LogAdd(MessError, "masterServer не смог занять порт")
		os.Exit(1)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			LogAdd(MessError, "masterServer не смог занять сокет")
			break
		}

		go Ping(&conn)
		go masterHandler(&conn)
	}

	ln.Close()
	LogAdd(MessInfo, "masterServer остановился")
}

func masterHandler(conn *net.Conn) {
	id := RandomString(MaxLengthIDLog)
	LogAdd(MessInfo, id+" masterServer получил соединение")

	var curNode Node

	reader := bufio.NewReader(*conn)

	for {
		buff, err := reader.ReadBytes('}')

		if err != nil {
			LogAdd(MessError, id+" ошибка чтения буфера")
			break
		}

		LogAdd(MessDetail, id+fmt.Sprint(" buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))

		//удаляем мусор
		if buff[0] != '{' {
			LogAdd(MessInfo, id+" masterServer удаляем мусор")
			if bytes.Index(buff, []byte("{")) >= 0 {
				buff = buff[bytes.Index(buff, []byte("{")):]
			} else {
				continue
			}
		}

		var message Message
		err = json.Unmarshal(buff, &message)
		if err != nil {
			LogAdd(MessError, id+" ошибка разбора json")
			time.Sleep(time.Millisecond * WaitIdle)
			continue
		}

		LogAdd(MessDetail, id+" "+fmt.Sprint(message))

		//обрабатываем полученное сообщение
		if len(ProcessingAgent) > message.TMessage {
			if ProcessingAgent[message.TMessage].Processing != nil {
				go ProcessingAgent[message.TMessage].Processing(message, conn, &curNode, id) //от одного агента может много приходить сообщений, не тормозим их
			} else {
				LogAdd(MessInfo, id+" нет обработчика для сообщения "+fmt.Sprint(message.TMessage))
				time.Sleep(time.Millisecond * WaitIdle)
			}
		} else {
			LogAdd(MessInfo, id+" неизвестное сообщение: "+fmt.Sprint(message.TMessage))
			time.Sleep(time.Millisecond * WaitIdle)
		}

	}
	(*conn).Close()

	//если есть id значит скорее всего есть в карте
	if len(curNode.Id) != 0 {
		nodes.Delete(curNode.Id)
		sendMessageToAllClients(TMESS_SERVERS, fmt.Sprint(false), curNode.Ip)
	}

	//удалим все сессии связанные с этим агентом
	channels.Range(func(key interface{}, value interface{}) bool {
		dConn := value.(*dConn)
		if dConn.node == &curNode {
			channels.Delete(key)
		}
		return true
	})

	LogAdd(MessInfo, id+" masterServer потерял соединение с агентом")
}

func NodeClient() {

	LogAdd(MessInfo, "nodeClient запустился")

	for {
		conn, err := net.Dial("tcp", Options.MasterServer+":"+Options.MasterPort)
		if err != nil {
			LogAdd(MessError, "nodeClient не смог подключиться: "+fmt.Sprint(err))
			time.Sleep(time.Second * WaitIdleAgent)
			continue
		}

		master = &conn

		hostname, err := os.Hostname()
		if err != nil {
			hostname = RandomString(MaxLengthIDNode)
		}
		if len(Options.Hostname) > 0 {
			hostname = Options.Hostname
		}
		sendMessage(&conn, TMESS_AGENT_AUTH, hostname, Options.MasterPassword, ReVisitVersion, fmt.Sprint(coordinates[0], ";", coordinates[1]))

		go Ping(&conn)

		reader := bufio.NewReader(conn)
		for {
			buff, err := reader.ReadBytes('}')

			if err != nil {
				LogAdd(MessError, "nodeClient ошибка чтения буфера: "+fmt.Sprint(err))
				break
			}

			LogAdd(MessDetail, fmt.Sprint("buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))

			//удаляем мусор
			if buff[0] != '{' {
				LogAdd(MessInfo, "nodeClient удаляем мусор")
				if bytes.Index(buff, []byte("{")) >= 0 {
					LogAdd(MessDetail, fmt.Sprint("buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))
					buff = buff[bytes.Index(buff, []byte("{")):]
				} else {
					continue
				}
			}

			var message Message
			err = json.Unmarshal(buff, &message)
			if err != nil {
				LogAdd(MessError, "nodeClient ошибка разбора json: "+fmt.Sprint(err))
				time.Sleep(time.Millisecond * WaitIdle)
				continue
			}

			LogAdd(MessDetail, fmt.Sprint(message))

			//обрабатываем полученное сообщение
			if len(ProcessingAgent) > message.TMessage {
				if ProcessingAgent[message.TMessage].Processing != nil {
					go ProcessingAgent[message.TMessage].Processing(message, &conn, nil, RandomString(MaxLengthIDLog))
				} else {
					LogAdd(MessInfo, "nodeClient нет обработчика для сообщения")
					time.Sleep(time.Millisecond * WaitIdle)
				}
			} else {
				LogAdd(MessInfo, "nodeClient неизвестное сообщение: "+fmt.Sprint(message.TMessage))
				time.Sleep(time.Millisecond * WaitIdle)
			}

		}
		conn.Close()
	}

	//LogAdd(MessInfo, "nodeClient остановился") //недостижимо???
}

func processAgentAuth(message Message, conn *net.Conn, curNode *Node, id string) {
	LogAdd(MessInfo, id+" пришла авторизация агента")

	if Options.Mode == ModeRegular {
		LogAdd(MessError, id+" режим не поддерживающий агентов")
		(*conn).Close()
		return
	}

	if Options.Mode == ModeNode {
		LogAdd(MessInfo, id+" пришел ответ на авторизацию")
		return
	}

	time.Sleep(time.Millisecond * WaitIdle)

	if len(message.Messages) < 3 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		(*conn).Close()
		return
	}

	if message.Messages[2] != ReVisitVersion {
		LogAdd(MessError, id+" не совместимая версия")
		(*conn).Close()
		return
	}

	if message.Messages[1] != Options.MasterPassword {
		LogAdd(MessError, id+" не правильный пароль")
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
			curNode.coordinates = GetCoordinatesByYandex(curNode.Ip)
		}()
	}

	curNode.Conn = conn
	curNode.Name = message.Messages[0]
	curNode.Id = RandomString(MaxLengthIDNode)

	h, _, err := net.SplitHostPort((*conn).RemoteAddr().String())
	if err == nil {
		curNode.Ip = h
	}

	if sendMessage(conn, TMESS_AGENT_AUTH, curNode.Id) {
		nodes.Store(curNode.Id, curNode)
		LogAdd(MessInfo, id+" авторизация агента успешна")
	}

	sendMessageToAllClients(TMESS_SERVERS, fmt.Sprint(true), curNode.Ip)
}

func processAgentAddCode(message Message, conn *net.Conn, curNode *Node, id string) {
	if Options.Mode != ModeNode {
		LogAdd(MessError, id+" режим не поддерживающий агентов")
		return
	}

	LogAdd(MessInfo, id+" пришла информация о создании сессии")

	if len(message.Messages) != 1 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return
	}

	connectPeers(message.Messages[0], nil, nil, "")
}

func processAgentDelCode(message Message, conn *net.Conn, curNode *Node, id string) {
	if Options.Mode == ModeRegular {
		LogAdd(MessError, id+" режим не поддерживающий агентов")
		return
	}

	LogAdd(MessInfo, id+" пришла информация об удалении сессии")

	if len(message.Messages) != 1 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return
	}

	disconnectPeers(message.Messages[0])
}

func processAgentAddBytes(message Message, conn *net.Conn, curNode *Node, id string) {
	if Options.Mode != ModeMaster {
		LogAdd(MessError, id+" режим не поддерживающий агентов")
		return
	}

	LogAdd(MessInfo, id+" пришла информация статистики")

	if len(message.Messages) != 1 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return
	}

	bytes, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		AddCounter(uint64(bytes))
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
	if Options.Mode != ModeMaster {
		LogAdd(MessError, id+" режим не поддерживающий агентов")
		return
	}

	LogAdd(MessInfo, id+" пришла информация о том что агент получил соединение")

	if len(message.Messages) != 1 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return
	}

	code := message.Messages[0]
	value, exists := channels.Load(code)
	if exists {
		peers := value.(*dConn)
		peers.node = curNode
		//отправим запрос принимающей стороне
		if !sendMessage(peers.client.Conn, TMESS_CONNECT, "", "", code, "simple", "client", peers.server.Pid, peers.address) {
			LogAdd(MessError, id+" не смогли отправить запрос принимающей стороне")
		}
	}
}

func processAgentPing(message Message, conn *net.Conn, curNode *Node, id string) {
	//LogAdd(MessInfo, id + " пришел пинг")
}
