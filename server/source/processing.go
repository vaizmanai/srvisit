package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func processVersion(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришла информация о версии")

	if len(message.Messages) != 1 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	curClient.Version = message.Messages[0]
}

func processAuth(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришла авторизация")

	if len(message.Messages) != 1 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}
	if len(message.Messages[0]) < 3 {
		time.Sleep(time.Millisecond * WAIT_IDLE)
		sendMessage(conn, TMESS_DEAUTH)
		logAdd(MESS_ERROR, id+" слабый serial")
		return
	}

	s := getPid(message.Messages[0])
	logAdd(MESS_INFO, id+" сгенерировали pid")

	salt := randomString(LEN_SALT)
	token := randomString(LEN_TOKEN)

	if sendMessage(conn, TMESS_AUTH, s, salt, token) {
		curClient.Conn = conn
		curClient.Pid = s
		curClient.Serial = message.Messages[0]
		curClient.Salt = salt
		curClient.Token = token
		curClient.storeClient()
		curClient.coordinates = [2]float64{0, 0}

		addClientToProfile(curClient)
		logAdd(MESS_INFO, id+" авторизация успешна")

		//получим координаты по ip
		go func() {
			h, _, err := net.SplitHostPort((*(*curClient).Conn).RemoteAddr().String())
			if err == nil {
				(*curClient).coordinates = getCoordinatesByYandex(h)
			}
		}()
	}
}

func processNotification(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" уведомление пришло")

	if len(message.Messages) != 2 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	list := clients[cleanPid(message.Messages[0])]

	if list != nil {
		//todo надо бы как-то защититься от спама
		for _, peer := range list {
			sendMessage(peer.Conn, TMESS_NOTIFICATION, message.Messages[1])
		}
	}
}

func processConnect(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" обрабатываем запрос на подключение")

	if len(message.Messages) < 2 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	salt := curClient.Salt
	if len(message.Messages) > 2 && len(message.Messages[2]) > 0 {
		salt = message.Messages[2]
	}

	var address string
	if len(message.Messages) > 3 && len(message.Messages[3]) > 0 {
		address = message.Messages[3]
	}

	list := clients[cleanPid(message.Messages[0])]

	successfully := false
	if list != nil && len(list) != 0 {
		passDigest := message.Messages[1]

		//отправим запрос на подключение всем, ответит только тот у кого пароль совпадет
		for _, peer := range list {
			code := randomString(CODE_LENGTH)

			//убедимся что версия клиента поддерживает соединения через агента
			if !greaterVersionThan(peer, MIN_VERSION_FOR_NODES) {
				address = ""
			}
			connectPeers(code, curClient, peer, address)

			logAdd(MESS_INFO, id+" запрашиваем коммуникацию у "+fmt.Sprint((*peer.Conn).RemoteAddr())+" для "+code)
			if !sendMessage(peer.Conn, TMESS_CONNECT, passDigest, salt, code, "simple", "server", curClient.Pid, address) { //тот кто передает трансляцию
				disconnectPeers(code)
				logAdd(MESS_ERROR, id+" не смогли отправить запрос "+code)
				if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
					sendMessage(curClient.Conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_NETWORK_ERROR))
				}
			}

			successfully = true
		}
	}

	if successfully {
		return
	}

	logAdd(MESS_INFO, id+" нет такого пира")
	sendMessage(curClient.Conn, TMESS_NOTIFICATION, "Нет такого пира") //todo удалить
	if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
		sendMessage(curClient.Conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_ABSENT_ERROR))
	}
}

func processDisconnect(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришел запрос на отключение")
	if len(message.Messages) < 1 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	code := message.Messages[0]

	if len(message.Messages) > 1 {
		i, err := strconv.Atoi(message.Messages[1])
		if err == nil {
			logAdd(MESS_ERROR, id+" текст ошибки: "+messStaticText[i])
			value, exists := channels.Load(code)
			if exists {
				peers := value.(*dConn)
				if greaterVersionThan(peers.client, MIN_VERSION_FOR_STATIC_ALERT) {
					sendMessage(peers.client.Conn, TMESS_STANDART_ALERT, message.Messages[1])
				}
			}
		}
	}

	disconnectPeers(code)
}

func processPing(message Message, conn *net.Conn, curClient *Client, id string) {
	//logAdd(MESS_INFO, id + " пришел пинг")
}

func processLogin(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришел запрос на авторизацию профиля")
	if len(message.Messages) != 2 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	email := strings.ToLower(message.Messages[0])
	profile, ok := profiles.Load(email)
	if ok == true {
		if message.Messages[1] == getSHA256(profile.(*Profile).Pass+curClient.Salt) {
			logAdd(MESS_INFO, id+" авторизация профиля пройдена")
			sendMessage(conn, TMESS_LOGIN)

			curClient.Profile = profile.(*Profile)
			profile.(*Profile).clients.Store(cleanPid(curClient.Pid), curClient)
			processContacts(message, conn, curClient, id)
			return
		}
	} else {
		logAdd(MESS_ERROR, id+" нет такой учетки")
	}

	logAdd(MESS_ERROR, id+" авторизация профиля не успешна")
	sendMessage(conn, TMESS_NOTIFICATION, "Авторизация профиля провалилась!") //todo удалить
	if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
		sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_AUTH_FAIL))
	}
}

func processReg(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришел запрос на регистрацию")
	if len(message.Messages) != 1 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	//проверяем доступность учетки
	_, ok := profiles.Load(message.Messages[0])
	if ok == false {
		newProfile := Profile{}
		newProfile.Email = strings.ToLower(message.Messages[0])
		if len(options.ServerSMTP) > 0 {
			newProfile.Pass = randomString(PASSWORD_LENGTH)

			msg := "Subject: Information from reVisit\r\n\r\nYour password is " + newProfile.Pass + "\r\n"

			success, err := sendEmail(message.Messages[0], msg)
			if !success {
				logAdd(MESS_ERROR, id+" не удалось отправить письмо с паролем: "+fmt.Sprint(err))
				sendMessage(conn, TMESS_NOTIFICATION, "Не удалось отправить письмо с паролем!") //todo удалить
				if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
					sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_REG_MAIL))
				}
				return
			}

			profiles.Store(newProfile.Email, &newProfile)
			sendMessage(conn, TMESS_REG, "success")
			sendMessage(conn, TMESS_NOTIFICATION, "Учетная запись создана, Ваш пароль на почте!") //todo удалить
			if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
				sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_REG_SUCCESSFUL))
			}
			logAdd(MESS_INFO, id+" создали учетку")
		} else {
			newProfile.Pass = PREDEFINED_PASS
		}

	} else {
		//todo восстановление пароля

		logAdd(MESS_INFO, id+" такая учетка уже существует")
		sendMessage(conn, TMESS_NOTIFICATION, "Такая учетная запись уже существует!") //todo удалить
		if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
			sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_REG_FAIL))
		}
	}

}

func processContact(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришел запрос на контакта")
	if len(message.Messages) != 6 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	profile := curClient.Profile
	if profile == nil {
		logAdd(MESS_ERROR, id+" не авторизован профиль")
		return
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		profile.mutex.Lock()
		defer profile.mutex.Unlock()

		if i == -1 {
			i = getNewId(profile.Contacts)
		}

		if message.Messages[1] == "del" {
			profile.Contacts = delContact(profile.Contacts, i) //удаляем ссылки на контакт
		} else {
			c := getContact(profile.Contacts, i)

			//если нет такого - создадим
			if c == nil {
				c = &Contact{}
				if len(message.Messages[5]) == 0 { //если не указан родитель, то в корень
					c.Next = profile.Contacts
					profile.Contacts = c
				}
			}

			if len(message.Messages[5]) > 0 { //поменяем родителя
				profile.Contacts = delContact(profile.Contacts, i) //удаляем ссылки на контакт

				ip, err := strconv.Atoi(message.Messages[5]) //IndexParent ищем нового родителя
				if err == nil {
					p := getContact(profile.Contacts, ip)
					if p != nil {
						c.Next = p.Inner
						p.Inner = c
					} else {
						c.Next = profile.Contacts
						profile.Contacts = c
					}
				} else {
					c.Next = profile.Contacts
					profile.Contacts = c
				}
			}

			c.Id = i
			c.Type = message.Messages[1]
			c.Caption = message.Messages[2]
			c.Pid = message.Messages[3]
			if len(message.Messages[4]) > 0 {
				c.Digest = message.Messages[4]
				c.Salt = curClient.Salt
			}
			message.Messages[0] = fmt.Sprint(i)

			//если такой пид онлайн - добавить наш профиль туда
			list := clients[cleanPid(message.Messages[3])]
			if list != nil {
				for _, peer := range list {
					peer.profiles.Store(profile.Email, profile)
				}
			}
		}

		//отправим всем авторизованным об изменениях
		profile.clients.Range(func(key interface{}, value interface{}) bool {
			sendMessage(value.(*Client).Conn, message.TMessage, message.Messages...)
			return true
		})

		processStatus(createMessage(TMESS_STATUS, fmt.Sprint(i)), conn, curClient, id)

		logAdd(MESS_INFO, id+" операция с контактом выполнена")
		return
	}
	logAdd(MESS_ERROR, id+" операция с контактом провалилась")
}

func processContacts(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришел запрос на обновления контактов")

	if curClient.Profile == nil {
		logAdd(MESS_ERROR, id+" профиль не авторизован")
	}

	//отправляем все контакты
	b, err := json.Marshal(curClient.Profile.Contacts)
	if err == nil {
		enc := url.PathEscape(string(b))
		sendMessage(conn, TMESS_CONTACTS, enc)
		logAdd(MESS_INFO, id+" отправили контакты")

		processStatuses(createMessage(TMESS_STATUSES), conn, curClient, id)
	} else {
		logAdd(MESS_ERROR, id+" не получилось отправить контакты: "+fmt.Sprint(err))
	}
}

func processLogout(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришел запрос на выход")

	if curClient.Profile == nil {
		logAdd(MESS_ERROR, id+" не авторизован профиль")
		return
	}

	curClient.Profile.clients.Delete(cleanPid(curClient.Pid))
	curClient.Profile = nil
}

func processConnectContact(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришел запрос на подключение к контакту")
	if len(message.Messages) < 1 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	profile := curClient.Profile
	if profile == nil {
		logAdd(MESS_ERROR, id+" не авторизован профиль")
		return
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		p := getContact(profile.Contacts, i)
		if p != nil {
			if len(message.Messages) > 1 {
				processConnect(createMessage(TMESS_CONNECT, p.Pid, p.Digest, p.Salt, message.Messages[1]), conn, curClient, id)
			} else {
				processConnect(createMessage(TMESS_CONNECT, p.Pid, p.Digest, p.Salt), conn, curClient, id)
			}
		} else {
			logAdd(MESS_ERROR, id+" нет такого контакта в профиле")
			sendMessage(conn, TMESS_NOTIFICATION, "Нет такого контакта в профиле!") //todo удалить
			if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
				sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_ABSENT_ERROR))
			}
		}
	} else {
		logAdd(MESS_ERROR, id+" ошибка преобразования идентификатора")
		sendMessage(conn, TMESS_NOTIFICATION, "Ошибка преобразования идентификатора!") //todo удалить
		if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
			sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_ABSENT_ERROR))
		}
	}
}

func processStatuses(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришел запрос на статусы профиля")
	if len(message.Messages) != 0 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	if curClient.Profile == nil {
		logAdd(MESS_ERROR, id+" не авторизован профиль")
		return
	}

	checkStatuses(curClient, curClient.Profile.Contacts)
}

func processStatus(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришел запрос на статус контакта")
	if len(message.Messages) != 1 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	if curClient.Profile == nil {
		logAdd(MESS_ERROR, id+" не авторизован профиль")
		return
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		contact := getContact(curClient.Profile.Contacts, i)
		if contact != nil {
			list := clients[cleanPid(contact.Pid)]
			if list != nil {
				sendMessage(conn, TMESS_STATUS, contact.Pid, "1")
			} else {
				sendMessage(conn, TMESS_STATUS, contact.Pid, "0")
			}
		}
	}
}

func processInfoContact(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришел запрос на информацию о контакте")
	if len(message.Messages) != 1 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	if curClient.Profile == nil {
		logAdd(MESS_ERROR, id+" не авторизован профиль")
		return
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		p := getContact(curClient.Profile.Contacts, i)
		if p != nil {
			list := clients[cleanPid(p.Pid)]
			if list != nil {
				for _, peer := range list {
					sendMessage(peer.Conn, TMESS_INFO_CONTACT, curClient.Pid, p.Digest, p.Salt)
				}
			} else {
				logAdd(MESS_ERROR, id+" нет такого контакта в сети")
				sendMessage(conn, TMESS_NOTIFICATION, "Нет такого контакта в сети!") //todo удалить
				if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
					sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_ABSENT_ERROR))
				}
			}
		} else {
			logAdd(MESS_ERROR, id+" нет такого контакта в профиле")
			sendMessage(conn, TMESS_NOTIFICATION, "Нет такого контакта в профиле!") //todo удалить
			if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
				sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_ABSENT_ERROR))
			}
		}
	} else {
		logAdd(MESS_ERROR, id+" ошибка преобразования идентификатора")
		sendMessage(conn, TMESS_NOTIFICATION, "Ошибка преобразования идентификатора!") //todo удалить
		if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
			sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_ABSENT_ERROR))
		}
	}

}

func processInfoAnswer(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришел ответ на информацию о контакте")
	if len(message.Messages) < 1 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	list := clients[cleanPid(message.Messages[0])]
	if list != nil {
		for _, peer := range list {
			if peer.Profile != nil {
				sendMessage(peer.Conn, TMESS_INFO_ANSWER, message.Messages...)
				logAdd(MESS_INFO, id+" вернули ответ")
			} else {
				logAdd(MESS_ERROR, id+" деавторизованный профиль")
			}
		}

	} else {
		logAdd(MESS_ERROR, id+" нет такого контакта в сети")
		sendMessage(conn, TMESS_NOTIFICATION, "Нет такого контакта в сети!") //todo удалить
		if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
			sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_ABSENT_ERROR))
		}
	}

}

func processManage(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришел запрос на управление")
	if len(message.Messages) < 2 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	if curClient.Profile == nil {
		logAdd(MESS_ERROR, id+" не авторизован профиль")
		return
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		p := getContact(curClient.Profile.Contacts, i)
		if p != nil {
			list := clients[cleanPid(p.Pid)]
			if list != nil {
				for _, peer := range list {
					var content []string
					content = append(content, curClient.Pid, p.Digest, p.Salt)
					content = append(content, message.Messages[1:]...)

					sendMessage(peer.Conn, TMESS_MANAGE, content...)
				}
			} else {
				logAdd(MESS_ERROR, id+" нет такого контакта в сети")
				sendMessage(conn, TMESS_NOTIFICATION, "Нет такого контакта в сети!") //todo удалить
				if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
					sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_ABSENT_ERROR))
				}
			}
		} else {
			logAdd(MESS_ERROR, id+" нет такого контакта в профиле")
			sendMessage(conn, TMESS_NOTIFICATION, "Нет такого контакта в профиле!") //todo удалить
			if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
				sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_ABSENT_ERROR))
			}
		}
	} else {
		logAdd(MESS_ERROR, id+" ошибка преобразования идентификатора")
		sendMessage(conn, TMESS_NOTIFICATION, "Ошибка преобразования идентификатора!") //todo удалить
		if greaterVersionThan(curClient, MIN_VERSION_FOR_STATIC_ALERT) {
			sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_ABSENT_ERROR))
		}
	}
}

func processContactReverse(message Message, conn *net.Conn, curClient *Client, id string) {
	logAdd(MESS_INFO, id+" пришел запрос на добавление в чужую учетку")

	if len(message.Messages) < 3 {
		logAdd(MESS_ERROR, id+" не правильное кол-во полей")
		return
	}

	//Message[0] - login profile
	//Message[1] - digest
	//Message[2] - caption

	value, exist := profiles.Load(message.Messages[0])
	if exist {
		curProfile := value.(*Profile)
		if getSHA256(curProfile.Pass+curClient.Salt) == message.Messages[1] {
			i := getNewId(curProfile.Contacts)

			c := &Contact{}
			c.Next = curProfile.Contacts //добавляем пока только в корень
			curProfile.Contacts = c

			c.Id = i
			c.Type = "node"
			c.Caption = message.Messages[2]
			c.Pid = curClient.Pid
			c.Digest = message.Messages[1]
			c.Salt = curClient.Salt

			//добавим этот профиль к авторизованному списку
			curClient.profiles.Store(curProfile.Email, curProfile)

			//отправим всем авторизованным об изменениях
			curProfile.clients.Range(func(key interface{}, value interface{}) bool {
				sendMessage(value.(*Client).Conn, TMESS_CONTACT, fmt.Sprint(i), "node", c.Caption, c.Pid, "", "-1")
				sendMessage(value.(*Client).Conn, TMESS_STATUS, fmt.Sprint(i), "1")
				return true
			})

			logAdd(MESS_INFO, id+" операция с контактом выполнена")
			return
		}
	}

	logAdd(MESS_ERROR, id+" не удалось добавить контакт в чужой профиль")
}

func processServers(message Message, conn *net.Conn, curClient *Client, id string) {
	//убедимся что версия клиента поддерживает соединения через агента
	if !greaterVersionThan(curClient, MIN_VERSION_FOR_NODES) {
		return
	}

	logAdd(MESS_INFO, id+" пришел запрос на информацию об агентах")

	if options.mode != MASTER {
		return
	}

	nodesString := make([]string, 0)
	nodes.Range(func(key interface{}, value interface{}) bool {
		nodesString = append(nodesString, value.(*Node).Ip)
		return true
	})

	sendMessage(conn, TMESS_SERVERS, nodesString...)
}
