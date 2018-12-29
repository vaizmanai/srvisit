package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime/debug"
	"strconv"
	"time"
)

func recoverMainServer(conn *net.Conn) {
	if recover() != nil {
		logAdd(MESS_ERROR, "Поток mainServer поймал критическую ошибку")
		debug.PrintStack()

		if conn != nil {
			(*conn).Close()
		}
	}
}

func recoverDataServer(conn *net.Conn) {
	if recover() != nil {
		logAdd(MESS_ERROR, "Поток dataServer поймал критическую ошибку")
		debug.PrintStack()

		if conn != nil {
			(*conn).Close()
		}
	}
}

func mainServer() {
	logAdd(MESS_INFO, "mainServer запустился")

	ln, err := net.Listen("tcp", ":"+options.MainServerPort)
	if err != nil {
		logAdd(MESS_ERROR, "mainServer не смог занять порт")
		os.Exit(1)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			logAdd(MESS_ERROR, "mainServer не смог занять сокет")
			break
		}

		go ping(&conn)
		go mainHandler(&conn)
	}

	ln.Close()
	logAdd(MESS_INFO, "mainServer остановился")
}

func mainHandler(conn *net.Conn) {
	id := randomString(MAX_LEN_ID_LOG)
	logAdd(MESS_INFO, id+" mainServer получил соединение "+fmt.Sprint((*conn).RemoteAddr()))

	defer recoverMainServer(conn)

	var curClient Client
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
			logAdd(MESS_INFO, id+" mainServer удаляем мусор")
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
		if len(processing) > message.TMessage {
			if processing[message.TMessage].Processing != nil {
				processing[message.TMessage].Processing(message, conn, &curClient, id)
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

	//удалим себя из профиля если авторизованы
	if curClient.Profile != nil {
		curClient.Profile.clients.Delete(cleanPid(curClient.Pid))
	}

	//пробежимся по профилям где мы есть и отправим новый статус
	curClient.profiles.Range(func(key interface{}, value interface{}) bool {
		profile := value.(*Profile)

		//все кто авторизовался в этот профиль должен получить новый статус
		profile.clients.Range(func(key interface{}, value interface{}) bool {
			client := value.(*Client)
			sendMessage(client.Conn, TMESS_STATUS, cleanPid(curClient.Pid), "0")
			return true
		})

		return true
	})

	//удалим себя из карты клиентов
	curClient.removeClient()

	logAdd(MESS_INFO, id+" mainServer потерял соединение с пиром "+fmt.Sprint((*conn).RemoteAddr()))
}

func dataServer() {
	logAdd(MESS_INFO, "dataServer запустился")

	ln, err := net.Listen("tcp", ":"+options.DataServerPort)
	if err != nil {
		logAdd(MESS_ERROR, "dataServer не смог занять порт")
		os.Exit(1)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			logAdd(MESS_ERROR, "dataServer не смог занять сокет")
			break
		}

		go dataHandler(&conn)
	}

	ln.Close()
	logAdd(MESS_INFO, "dataServer остановился")
}

func dataHandler(conn *net.Conn) {
	id := randomString(6)
	logAdd(MESS_INFO, id+" dataHandler получил соединение "+fmt.Sprint((*conn).RemoteAddr()))

	defer recoverDataServer(conn)

	for {
		code, err := bufio.NewReader(*conn).ReadString('\n')

		if err != nil {
			logAdd(MESS_ERROR, id+" ошибка чтения кода")
			break
		}

		code = code[:len(code)-1]
		value, exist := channels.Load(code)
		if exist == false {
			logAdd(MESS_ERROR, id+" не ожидаем такого кода")
			break
		}

		peers := value.(*dConn)
		peers.mutex.Lock()
		var numPeer int
		if peers.pointer[0] == nil {
			peers.pointer[0] = conn
			numPeer = 1

			if options.mode == REGULAR {
				//отправим запрос принимающей стороне
				if !sendMessage(peers.client.Conn, TMESS_CONNECT, "", "", code, "simple", "client", peers.server.Pid, peers.address) {
					logAdd(MESS_ERROR, id+" не смогли отправить запрос принимающей стороне")
				}
			} else { //options.mode == NODE
				sendMessageToMaster(TMESS_AGENT_NEW_CONN, code) //оповестим мастер о том что мы дождались транслятор
			}

		} else if peers.pointer[1] == nil {
			peers.pointer[1] = conn
			numPeer = 0
		}
		peers.mutex.Unlock()

		var cWait = 0
		for peers.pointer[numPeer] == nil && cWait < WAIT_COUNT {
			logAdd(MESS_INFO, id+" ожидаем пира для "+code)
			time.Sleep(time.Millisecond * WAIT_IDLE)
			cWait++
		}

		if peers.pointer[numPeer] == nil {
			logAdd(MESS_ERROR, id+" превышено время ожидания")
			channels.Delete(code)
			break
		}

		logAdd(MESS_INFO, id+" пир существует для "+code)
		time.Sleep(time.Millisecond * WAIT_AFTER_CONNECT)

		var z []byte
		z = make([]byte, options.SizeBuff)

		var countBytes uint64
		var n1, n2 int
		var err1, err2 error

		for {
			n1, err1 = (*conn).Read(z)

			if peers.pointer[numPeer] == nil {
				logAdd(MESS_INFO, id+" потеряли пир")
				time.Sleep(time.Millisecond * WAIT_AFTER_CONNECT)
				break
			}

			n2, err2 = (*peers.pointer[numPeer]).Write(z[:n1])

			countBytes = countBytes + uint64(n1+n2)

			if err1 != nil || err2 != nil || n1 == 0 || n2 == 0 {
				logAdd(MESS_INFO, id+" соединение закрылось: "+fmt.Sprint(n1, n2))
				logAdd(MESS_INFO, id+" err1: "+fmt.Sprint(err1))
				logAdd(MESS_INFO, id+" err2: "+fmt.Sprint(err2))
				time.Sleep(time.Millisecond * WAIT_AFTER_CONNECT)
				if peers.pointer[numPeer] != nil {
					(*peers.pointer[numPeer]).Close()
				}
				break
			}
		}

		addCounter(countBytes)
		if options.mode == NODE {
			sendMessageToMaster(TMESS_AGENT_ADD_BYTES, fmt.Sprint(countBytes))
		}

		logAdd(MESS_INFO, id+" поток завершается")

		disconnectPeers(code)

		break
	}
	(*conn).Close()
	logAdd(MESS_INFO, id+" dataHandler потерял соединение")

}

func disconnectPeers(code string) {
	value, exists := channels.Load(code)
	if exists {
		channels.Delete(code)
		pair := value.(*dConn)

		if options.mode != MASTER {
			if pair.pointer[0] != nil {
				(*pair.pointer[0]).Close()
			}
			if pair.pointer[1] != nil {
				(*pair.pointer[1]).Close()
			}
		}
		if options.mode == MASTER {
			sendMessageToNodes(TMESS_AGENT_DEL_CODE, code)
		}
		if options.mode == NODE {
			sendMessageToMaster(TMESS_AGENT_DEL_CODE, code)
		}

		pair.client = nil
		pair.server = nil
	}
}

func connectPeers(code string, client *Client, server *Client, address string) {
	var newConnection dConn
	channels.Store(code, &newConnection)
	newConnection.client = client
	newConnection.server = server
	newConnection.address = address

	go checkConnection(&newConnection, code) //может случиться так, что код сохранили, а никто не подключился

	if options.mode == MASTER {
		sendMessageToNodes(TMESS_AGENT_ADD_CODE, code)
	}
}

func checkConnection(connection *dConn, code string) {
	time.Sleep(time.Second * WAIT_CONNECTION)

	if options.mode != NODE {
		if connection.node == nil && connection.pointer[0] == nil && connection.pointer[1] == nil {
			logAdd(MESS_ERROR, "таймаут ожидания соединений для "+code)
			if connection.client != nil {
				if greaterVersionThan(connection.client, MIN_VERSION_FOR_STATIC_ALERT) {
					sendMessage(connection.client.Conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_TIMEOUT_ERROR))
				}
			}
			disconnectPeers(code)
		}
	} else {
		if (connection.pointer[0] != nil && connection.pointer[1] == nil) || (connection.pointer[0] == nil && connection.pointer[1] != nil) {
			logAdd(MESS_ERROR, "таймаут ожидания соединений для "+code)
			disconnectPeers(code)
		}
	}
}
