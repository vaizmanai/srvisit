package service

import (
	. "../common"
	. "../component/client"
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
		LogAdd(MessError, "поток mainServer поймал критическую ошибку")
		debug.PrintStack()

		if conn != nil {
			(*conn).Close()
		}
	}
}

func recoverDataServer(conn *net.Conn) {
	if recover() != nil {
		LogAdd(MessError, "поток dataServer поймал критическую ошибку")
		debug.PrintStack()

		if conn != nil {
			(*conn).Close()
		}
	}
}

func MainServer() {
	LogAdd(MessInfo, "mainServer запустился")

	ln, err := net.Listen("tcp", ":"+Options.MainServerPort)
	if err != nil {
		LogAdd(MessError, "mainServer не смог занять порт")
		os.Exit(1)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			LogAdd(MessError, "mainServer не смог занять сокет")
			break
		}

		go Ping(&conn)
		go mainHandler(&conn)
	}

	ln.Close()
	LogAdd(MessInfo, "mainServer остановился")
}

func mainHandler(conn *net.Conn) {
	id := RandomString(MaxLengthIDLog)
	LogAdd(MessInfo, id+" mainServer получил соединение "+fmt.Sprint((*conn).RemoteAddr()))

	defer recoverMainServer(conn)

	var curClient Client
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
			LogAdd(MessInfo, id+" mainServer удаляем мусор")
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
		if len(Processing) > message.TMessage {
			if Processing[message.TMessage].Processing != nil {
				Processing[message.TMessage].Processing(message, conn, &curClient, id)
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

	//удалим связи с профилем
	if curClient.Profile != nil {
		DelAuthorizedClient(curClient.Profile.Email, &curClient)
		DelContainedProfile(curClient.Pid, curClient.Profile)
	}

	//пробежимся по профилям где мы есть и отправим новый статус
	for _, profile := range GetContainedProfileList(curClient.Pid) {
		//все кто авторизовался в этот профиль должен получить новый статус
		for _, client := range GetAuthorizedClientList(profile.Email) {
			sendMessage(client.Conn, TMESS_STATUS, CleanPid(curClient.Pid), "0")
		}
	}

	//удалим себя из карты клиентов
	curClient.RemoveClient()

	LogAdd(MessInfo, id+" mainServer потерял соединение с пиром "+fmt.Sprint((*conn).RemoteAddr()))
}

func DataServer() {
	LogAdd(MessInfo, "dataServer запустился")

	ln, err := net.Listen("tcp", ":"+Options.DataServerPort)
	if err != nil {
		LogAdd(MessError, "dataServer не смог занять порт")
		os.Exit(1)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			LogAdd(MessError, "dataServer не смог занять сокет")
			break
		}

		go dataHandler(&conn)
	}

	ln.Close()
	LogAdd(MessInfo, "dataServer остановился")
}

func dataHandler(conn *net.Conn) {
	id := RandomString(6)
	LogAdd(MessInfo, id+" dataHandler получил соединение "+fmt.Sprint((*conn).RemoteAddr()))

	defer recoverDataServer(conn)

	for {
		code, err := bufio.NewReader(*conn).ReadString('\n')

		if err != nil {
			LogAdd(MessError, id+" ошибка чтения кода")
			break
		}

		code = code[:len(code)-1]
		value, exist := channels.Load(code)
		if exist == false {
			LogAdd(MessError, id+" не ожидаем такого кода")
			break
		}

		peers := value.(*dConn)
		peers.mutex.Lock()
		var numPeer int
		if peers.pointer[0] == nil {
			peers.pointer[0] = conn
			numPeer = 1

			if Options.Mode == ModeRegular {
				//отправим запрос принимающей стороне
				if !sendMessage(peers.client.Conn, TMESS_CONNECT, "", "", code, "simple", "client", peers.server.Pid, peers.address) {
					LogAdd(MessError, id+" не смогли отправить запрос принимающей стороне")
				}
			} else { //options.mode == ModeNode
				sendMessageToMaster(TMESS_AGENT_NEW_CONN, code) //оповестим мастер о том что мы дождались транслятор
			}

		} else if peers.pointer[1] == nil {
			peers.pointer[1] = conn
			numPeer = 0
		}
		peers.mutex.Unlock()

		var cWait = 0
		for peers.pointer[numPeer] == nil && cWait < WaitCount {
			LogAdd(MessInfo, id+" ожидаем пира для "+code)
			time.Sleep(time.Millisecond * WaitIdle)
			cWait++
		}

		if peers.pointer[numPeer] == nil {
			LogAdd(MessError, id+" превышено время ожидания")
			disconnectPeers(code)
			break
		}

		LogAdd(MessInfo, id+" пир существует для "+code)
		time.Sleep(time.Millisecond * WaitAfterConnect)

		var z []byte
		z = make([]byte, Options.SizeBuff)

		var countBytes uint64
		var n1, n2 int
		var err1, err2 error

		for {
			n1, err1 = (*conn).Read(z)

			if peers.pointer[numPeer] == nil {
				LogAdd(MessInfo, id+" потеряли пир")
				time.Sleep(time.Millisecond * WaitAfterConnect)
				break
			}

			n2, err2 = (*peers.pointer[numPeer]).Write(z[:n1])

			countBytes = countBytes + uint64(n1+n2)

			if err1 != nil || err2 != nil || n1 == 0 || n2 == 0 {
				LogAdd(MessInfo, id+" соединение закрылось: "+fmt.Sprint(n1, n2))
				LogAdd(MessInfo, id+" err1: "+fmt.Sprint(err1))
				LogAdd(MessInfo, id+" err2: "+fmt.Sprint(err2))
				time.Sleep(time.Millisecond * WaitAfterConnect)
				if peers.pointer[numPeer] != nil {
					(*peers.pointer[numPeer]).Close()
				}
				break
			}
		}

		AddCounter(countBytes)
		if Options.Mode == ModeNode {
			sendMessageToMaster(TMESS_AGENT_ADD_BYTES, fmt.Sprint(countBytes))
		}

		LogAdd(MessInfo, id+" поток завершается")
		disconnectPeers(code)
		break
	}
	(*conn).Close()
	LogAdd(MessInfo, id+" dataHandler потерял соединение")

}

func disconnectPeers(code string) {
	value, exists := channels.Load(code)
	if exists {
		channels.Delete(code)
		pair := value.(*dConn)

		if Options.Mode != ModeMaster {
			if pair.pointer[0] != nil {
				(*pair.pointer[0]).Close()
			}
			if pair.pointer[1] != nil {
				(*pair.pointer[1]).Close()
			}
		}
		if Options.Mode == ModeMaster {
			sendMessageToNodes(TMESS_AGENT_DEL_CODE, code)
		}
		if Options.Mode == ModeNode {
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

	if Options.Mode == ModeMaster {
		sendMessageToNodes(TMESS_AGENT_ADD_CODE, code)
	}
}

func checkConnection(connection *dConn, code string) {
	time.Sleep(time.Second * WaitConnection)

	if Options.Mode != ModeNode {
		if connection.node == nil && connection.pointer[0] == nil && connection.pointer[1] == nil {
			LogAdd(MessError, "таймаут ожидания соединений для "+code)
			if connection.client != nil {
				if connection.client.GreaterVersionThan(MinimalVersionForStaticAlert) {
					sendMessage(connection.client.Conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageTimeoutError))
				}
			}
			disconnectPeers(code)
		}
	} else {
		if (connection.pointer[0] != nil && connection.pointer[1] == nil) || (connection.pointer[0] == nil && connection.pointer[1] != nil) {
			LogAdd(MessError, "таймаут ожидания соединений для "+code)
			disconnectPeers(code)
		}
	}
}
