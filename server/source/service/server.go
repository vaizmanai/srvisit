package service

import (
	. "../common"
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
		LogAdd(MESS_ERROR, "поток mainServer поймал критическую ошибку")
		debug.PrintStack()

		if conn != nil {
			(*conn).Close()
		}
	}
}

func recoverDataServer(conn *net.Conn) {
	if recover() != nil {
		LogAdd(MESS_ERROR, "поток dataServer поймал критическую ошибку")
		debug.PrintStack()

		if conn != nil {
			(*conn).Close()
		}
	}
}

func MainServer() {
	LogAdd(MESS_INFO, "mainServer запустился")

	ln, err := net.Listen("tcp", ":"+Options.MainServerPort)
	if err != nil {
		LogAdd(MESS_ERROR, "mainServer не смог занять порт")
		os.Exit(1)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			LogAdd(MESS_ERROR, "mainServer не смог занять сокет")
			break
		}

		go Ping(&conn)
		go mainHandler(&conn)
	}

	ln.Close()
	LogAdd(MESS_INFO, "mainServer остановился")
}

func mainHandler(conn *net.Conn) {
	id := RandomString(MAX_LEN_ID_LOG)
	LogAdd(MESS_INFO, id+" mainServer получил соединение "+fmt.Sprint((*conn).RemoteAddr()))

	defer recoverMainServer(conn)

	var curClient Client
	reader := bufio.NewReader(*conn)

	for {
		buff, err := reader.ReadBytes('}')

		if err != nil {
			LogAdd(MESS_ERROR, id+" ошибка чтения буфера")
			break
		}

		LogAdd(MESS_DETAIL, id+fmt.Sprint(" buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))

		//удаляем мусор
		if buff[0] != '{' {
			LogAdd(MESS_INFO, id+" mainServer удаляем мусор")
			if bytes.Index(buff, []byte("{")) >= 0 {
				buff = buff[bytes.Index(buff, []byte("{")):]
			} else {
				continue
			}
		}

		var message Message
		err = json.Unmarshal(buff, &message)
		if err != nil {
			LogAdd(MESS_ERROR, id+" ошибка разбора json")
			time.Sleep(time.Millisecond * WAIT_IDLE)
			continue
		}

		LogAdd(MESS_DETAIL, id+" "+fmt.Sprint(message))

		//обрабатываем полученное сообщение
		if len(Processing) > message.TMessage {
			if Processing[message.TMessage].Processing != nil {
				Processing[message.TMessage].Processing(message, conn, &curClient, id)
			} else {
				LogAdd(MESS_INFO, id+" нет обработчика для сообщения "+fmt.Sprint(message.TMessage))
				time.Sleep(time.Millisecond * WAIT_IDLE)
			}
		} else {
			LogAdd(MESS_INFO, id+" неизвестное сообщение: "+fmt.Sprint(message.TMessage))
			time.Sleep(time.Millisecond * WAIT_IDLE)
		}

	}
	(*conn).Close()

	//удалим себя из профиля если авторизованы
	if curClient.Profile != nil {
		curClient.Profile.clients.Delete(CleanPid(curClient.Pid))
	}

	//пробежимся по профилям где мы есть и отправим новый статус
	curClient.profiles.Range(func(key interface{}, value interface{}) bool {
		profile := value.(*Profile)

		//все кто авторизовался в этот профиль должен получить новый статус
		profile.clients.Range(func(key interface{}, value interface{}) bool {
			client := value.(*Client)
			sendMessage(client.Conn, TMESS_STATUS, CleanPid(curClient.Pid), "0")
			return true
		})

		return true
	})

	//удалим себя из карты клиентов
	curClient.removeClient()

	LogAdd(MESS_INFO, id+" mainServer потерял соединение с пиром "+fmt.Sprint((*conn).RemoteAddr()))
}

func DataServer() {
	LogAdd(MESS_INFO, "dataServer запустился")

	ln, err := net.Listen("tcp", ":"+Options.DataServerPort)
	if err != nil {
		LogAdd(MESS_ERROR, "dataServer не смог занять порт")
		os.Exit(1)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			LogAdd(MESS_ERROR, "dataServer не смог занять сокет")
			break
		}

		go dataHandler(&conn)
	}

	ln.Close()
	LogAdd(MESS_INFO, "dataServer остановился")
}

func dataHandler(conn *net.Conn) {
	id := RandomString(6)
	LogAdd(MESS_INFO, id+" dataHandler получил соединение "+fmt.Sprint((*conn).RemoteAddr()))

	defer recoverDataServer(conn)

	for {
		code, err := bufio.NewReader(*conn).ReadString('\n')

		if err != nil {
			LogAdd(MESS_ERROR, id+" ошибка чтения кода")
			break
		}

		code = code[:len(code)-1]
		value, exist := channels.Load(code)
		if exist == false {
			LogAdd(MESS_ERROR, id+" не ожидаем такого кода")
			break
		}

		peers := value.(*dConn)
		peers.mutex.Lock()
		var numPeer int
		if peers.pointer[0] == nil {
			peers.pointer[0] = conn
			numPeer = 1

			if Options.Mode == REGULAR {
				//отправим запрос принимающей стороне
				if !sendMessage(peers.client.Conn, TMESS_CONNECT, "", "", code, "simple", "client", peers.server.Pid, peers.address) {
					LogAdd(MESS_ERROR, id+" не смогли отправить запрос принимающей стороне")
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
			LogAdd(MESS_INFO, id+" ожидаем пира для "+code)
			time.Sleep(time.Millisecond * WAIT_IDLE)
			cWait++
		}

		if peers.pointer[numPeer] == nil {
			LogAdd(MESS_ERROR, id+" превышено время ожидания")
			disconnectPeers(code)
			break
		}

		LogAdd(MESS_INFO, id+" пир существует для "+code)
		time.Sleep(time.Millisecond * WAIT_AFTER_CONNECT)

		var z []byte
		z = make([]byte, Options.SizeBuff)

		var countBytes uint64
		var n1, n2 int
		var err1, err2 error

		for {
			n1, err1 = (*conn).Read(z)

			if peers.pointer[numPeer] == nil {
				LogAdd(MESS_INFO, id+" потеряли пир")
				time.Sleep(time.Millisecond * WAIT_AFTER_CONNECT)
				break
			}

			n2, err2 = (*peers.pointer[numPeer]).Write(z[:n1])

			countBytes = countBytes + uint64(n1+n2)

			if err1 != nil || err2 != nil || n1 == 0 || n2 == 0 {
				LogAdd(MESS_INFO, id+" соединение закрылось: "+fmt.Sprint(n1, n2))
				LogAdd(MESS_INFO, id+" err1: "+fmt.Sprint(err1))
				LogAdd(MESS_INFO, id+" err2: "+fmt.Sprint(err2))
				time.Sleep(time.Millisecond * WAIT_AFTER_CONNECT)
				if peers.pointer[numPeer] != nil {
					(*peers.pointer[numPeer]).Close()
				}
				break
			}
		}

		AddCounter(countBytes)
		if Options.Mode == NODE {
			sendMessageToMaster(TMESS_AGENT_ADD_BYTES, fmt.Sprint(countBytes))
		}

		LogAdd(MESS_INFO, id+" поток завершается")
		disconnectPeers(code)
		break
	}
	(*conn).Close()
	LogAdd(MESS_INFO, id+" dataHandler потерял соединение")

}

func disconnectPeers(code string) {
	value, exists := channels.Load(code)
	if exists {
		channels.Delete(code)
		pair := value.(*dConn)

		if Options.Mode != MASTER {
			if pair.pointer[0] != nil {
				(*pair.pointer[0]).Close()
			}
			if pair.pointer[1] != nil {
				(*pair.pointer[1]).Close()
			}
		}
		if Options.Mode == MASTER {
			sendMessageToNodes(TMESS_AGENT_DEL_CODE, code)
		}
		if Options.Mode == NODE {
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

	if Options.Mode == MASTER {
		sendMessageToNodes(TMESS_AGENT_ADD_CODE, code)
	}
}

func checkConnection(connection *dConn, code string) {
	time.Sleep(time.Second * WAIT_CONNECTION)

	if Options.Mode != NODE {
		if connection.node == nil && connection.pointer[0] == nil && connection.pointer[1] == nil {
			LogAdd(MESS_ERROR, "таймаут ожидания соединений для "+code)
			if connection.client != nil {
				if greaterVersionThan(connection.client, MIN_VERSION_FOR_STATIC_ALERT) {
					sendMessage(connection.client.Conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_TIMEOUT_ERROR))
				}
			}
			disconnectPeers(code)
		}
	} else {
		if (connection.pointer[0] != nil && connection.pointer[1] == nil) || (connection.pointer[0] == nil && connection.pointer[1] != nil) {
			LogAdd(MESS_ERROR, "таймаут ожидания соединений для "+code)
			disconnectPeers(code)
		}
	}
}
