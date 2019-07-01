package service

import (
	. "../common"
	. "../component/client"
	. "../component/contact"
	. "../component/profile"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func processVersion(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришла информация о версии")

	if len(message.Messages) != 1 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	curClient.Version = message.Messages[0]
	return true
}

func processAuth(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришла авторизация")

	if len(message.Messages) != 1 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}
	if len(message.Messages[0]) < 3 {
		time.Sleep(time.Millisecond * WaitIdle)
		sendMessage(conn, TMESS_DEAUTH)
		LogAdd(MessError, id+" слабый serial")
		return false
	}

	s := GetPid(message.Messages[0])
	LogAdd(MessInfo, id+" сгенерировали pid")

	salt := RandomString(LengthSalt)
	token := RandomString(LengthToken)

	if sendMessage(conn, TMESS_AUTH, s, salt, token) {
		curClient.Conn = conn
		curClient.Pid = s
		curClient.Serial = message.Messages[0]
		curClient.Salt = salt
		curClient.Token = token
		curClient.StoreClient()
		curClient.SetCoordinates([2]float64{0, 0})

		addClientToProfile(curClient)
		LogAdd(MessInfo, id+" авторизация успешна")

		//получим координаты по ip
		go func() {
			h, _, err := net.SplitHostPort((*curClient.Conn).RemoteAddr().String())
			if err == nil {
				curClient.SetCoordinates(GetCoordinatesByYandex(h))
			}
		}()
	}

	return true
}

func processNotification(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" уведомление пришло")

	if len(message.Messages) != 2 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	//todo надо бы как-то защититься от спама
	list := GetClientsList(message.Messages[0])
	for _, peer := range list {
		sendMessage(peer.Conn, TMESS_NOTIFICATION, message.Messages[1])
	}

	return true
}

func processConnect(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" обрабатываем запрос на подключение")

	if len(message.Messages) < 2 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	salt := curClient.Salt
	if len(message.Messages) > 2 && len(message.Messages[2]) > 0 {
		salt = message.Messages[2]
	}

	var address string
	if len(message.Messages) > 3 && len(message.Messages[3]) > 0 {
		address = message.Messages[3]
	}

	list := GetClientsList(message.Messages[0])

	successfully := false
	passDigest := message.Messages[1]

	//отправим запрос на подключение всем, ответит только тот у кого пароль совпадет
	for _, peer := range list {
		code := RandomString(CodeLength)

		//убедимся что версия клиента поддерживает соединения через агента
		if !peer.GreaterVersionThan(MinimalVersionForNodes) {
			address = ""
		}
		connectPeers(code, curClient, peer, address)

		LogAdd(MessInfo, id+" запрашиваем коммуникацию у "+fmt.Sprint((*peer.Conn).RemoteAddr())+" для "+code)
		if !sendMessage(peer.Conn, TMESS_CONNECT, passDigest, salt, code, "simple", "server", curClient.Pid, address) { //тот кто передает трансляцию
			disconnectPeers(code)
			LogAdd(MessError, id+" не смогли отправить запрос "+code)
			if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
				sendMessage(curClient.Conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageNetworkError))
			}
		}

		successfully = true
	}

	if successfully {
		return true
	}

	LogAdd(MessInfo, id+" нет такого пира")
	if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
		sendMessage(curClient.Conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageAbsentError))
	} else {
		sendMessage(curClient.Conn, TMESS_NOTIFICATION, "Нет такого пира") //todo удалить
	}

	return false
}

func processDisconnect(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришел запрос на отключение")
	if len(message.Messages) < 1 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	code := message.Messages[0]
	if len(code) == 0 {
		return false
	}

	if len(message.Messages) > 1 {
		i, err := strconv.Atoi(message.Messages[1])
		if err == nil {
			LogAdd(MessError, id+" текст ошибки: "+messStaticText[i])
			value, exists := channels.Load(code)
			if exists {
				peers := value.(*dConn)
				if peers.client.GreaterVersionThan(MinimalVersionForStaticAlert) {
					sendMessage(peers.client.Conn, TMESS_STANDART_ALERT, message.Messages[1])
				}
			}
		}
	}

	disconnectPeers(code)
	return true
}

func processPing(message Message, conn *net.Conn, curClient *Client, id string) bool {
	//LogAdd(MessInfo, id + " пришел пинг")
	return true
}

func processLogin(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришел запрос на авторизацию профиля")
	if len(message.Messages) != 2 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	email := strings.ToLower(message.Messages[0])
	profile := GetProfile(email)
	if profile != nil {
		if message.Messages[1] == GetSHA256(profile.Pass+curClient.Salt) {
			LogAdd(MessInfo, id+" авторизация профиля пройдена")

			sendMessage(conn, TMESS_LOGIN)
			curClient.Profile = profile
			AddAuthorizedClient(profile.Email, curClient)
			processContacts(message, conn, curClient, id)
			return true
		}
	} else {
		LogAdd(MessError, id+" нет такой учетки")
	}

	LogAdd(MessError, id+" авторизация профиля не успешна")
	if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
		sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageAuthFail))
	} else {
		sendMessage(conn, TMESS_NOTIFICATION, "Авторизация профиля провалилась!") //todo удалить
	}
	return true
}

func processReg(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришел запрос на регистрацию")
	if len(message.Messages) != 1 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	//проверяем доступность учетки
	profile := GetProfile(message.Messages[0])
	if profile == nil {
		newProfile := NewProfile(strings.ToLower(message.Messages[0]))

		if len(Options.ServerSMTP) > 0 {
			newProfile.Pass = RandomString(PasswordLength)
			msg := "Subject: Information from reVisit\r\n\r\nYour password is " + newProfile.Pass + "\r\n"
			success, err := SendEmail(message.Messages[0], msg)
			if !success {
				DelProfile(newProfile.Email)
				LogAdd(MessError, id+" не удалось отправить письмо с паролем: "+fmt.Sprint(err))
				if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
					sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageRegMail))
				} else {
					sendMessage(conn, TMESS_NOTIFICATION, "Не удалось отправить письмо с паролем!") //todo удалить
				}
				return false
			}
		} else {
			newProfile.Pass = PredefinedPass
		}

		sendMessage(conn, TMESS_REG, "success")
		if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
			sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageRegSuccessful))
		} else {
			sendMessage(conn, TMESS_NOTIFICATION, "Учетная запись создана, Ваш пароль на почте!") //todo удалить
		}
		LogAdd(MessInfo, id+" создали учетку")

	} else {
		//todo восстановление пароля

		LogAdd(MessInfo, id+" такая учетка уже существует")
		if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
			sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageRegFail))
		} else {
			sendMessage(conn, TMESS_NOTIFICATION, "Такая учетная запись уже существует!") //todo удалить
		}
	}
	return true
}

func processContact(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришел запрос на контакта")
	if len(message.Messages) != 6 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	profile := curClient.Profile
	if profile == nil {
		LogAdd(MessError, id+" не авторизован профиль")
		return false
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		profile.Lock()
		defer profile.Unlock()

		if i == -1 {
			i = GetNewId(profile.Contacts)
		}

		if message.Messages[1] == "del" {
			profile.Contacts = DelContact(profile.Contacts, i) //удаляем ссылки на контакт
		} else {
			c := GetContact(profile.Contacts, i)

			//если нет такого - создадим
			if c == nil {
				c = &Contact{}
				if len(message.Messages[5]) == 0 { //если не указан родитель, то в корень
					c.Next = profile.Contacts
					profile.Contacts = c
				}
			}

			if len(message.Messages[5]) > 0 { //поменяем родителя
				profile.Contacts = DelContact(profile.Contacts, i) //удаляем ссылки на контакт

				ip, err := strconv.Atoi(message.Messages[5]) //IndexParent ищем нового родителя
				if err == nil {

					p := GetContact(profile.Contacts, ip)

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
			list := GetClientsList(message.Messages[3])
			for _, peer := range list {
				AddContainedProfile(peer.Pid, profile)
			}

			if len(c.Pid) > 0 {
				processStatus(createMessage(TMESS_STATUS, fmt.Sprint(i)), conn, curClient, id)
			}
		}

		//отправим всем авторизованным об изменениях
		for _, authClient := range GetAuthorizedClientList(profile.Email) {
			sendMessage(authClient.Conn, message.TMessage, message.Messages...)
		}

		LogAdd(MessInfo, id+" операция с контактом выполнена")
		return true
	}
	LogAdd(MessError, id+" операция с контактом провалилась")
	return false
}

func processContacts(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришел запрос на обновления контактов")

	if curClient.Profile == nil {
		LogAdd(MessError, id+" профиль не авторизован")
		return false
	}

	//отправляем все контакты
	b, err := json.Marshal(curClient.Profile.Contacts)
	if err != nil {
		LogAdd(MessError, id+" не получилось отправить контакты: "+fmt.Sprint(err))
		return false
	}

	enc := url.PathEscape(string(b))
	sendMessage(conn, TMESS_CONTACTS, enc)
	LogAdd(MessInfo, id+" отправили контакты")

	processStatuses(createMessage(TMESS_STATUSES), conn, curClient, id)
	return true
}

func processLogout(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришел запрос на выход")

	if curClient.Profile == nil {
		LogAdd(MessError, id+" не авторизован профиль")
		return false
	}

	DelAuthorizedClient(curClient.Profile.Email, curClient)
	curClient.Profile = nil
	return true
}

func processConnectContact(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришел запрос на подключение к контакту")
	if len(message.Messages) < 1 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	profile := curClient.Profile
	if profile == nil {
		LogAdd(MessError, id+" не авторизован профиль")
		return false
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		p := GetContact(profile.Contacts, i)
		if p != nil {
			if len(message.Messages) > 1 {
				processConnect(createMessage(TMESS_CONNECT, p.Pid, p.Digest, p.Salt, message.Messages[1]), conn, curClient, id)
			} else {
				processConnect(createMessage(TMESS_CONNECT, p.Pid, p.Digest, p.Salt), conn, curClient, id)
			}
		} else {
			LogAdd(MessError, id+" нет такого контакта в профиле")
			if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
				sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageAbsentError))
			} else {
				sendMessage(conn, TMESS_NOTIFICATION, "Нет такого контакта в профиле!") //todo удалить
			}
		}
	} else {
		LogAdd(MessError, id+" ошибка преобразования идентификатора")
		if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
			sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageAbsentError))
		} else {
			sendMessage(conn, TMESS_NOTIFICATION, "Ошибка преобразования идентификатора!") //todo удалить
		}
	}
	return true
}

func processStatuses(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришел запрос на статусы профиля")
	if len(message.Messages) != 0 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	if curClient.Profile == nil {
		LogAdd(MessError, id+" не авторизован профиль")
		return false
	}

	checkStatuses(curClient, curClient.Profile.Contacts)
	return true
}

func processStatus(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришел запрос на статус контакта")
	if len(message.Messages) != 1 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	if curClient.Profile == nil {
		LogAdd(MessError, id+" не авторизован профиль")
		return false
	}

	if len(message.Messages[0]) == 0 {
		LogAdd(MessError, id+" пустой индекс")
		return false
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		contact := GetContact(curClient.Profile.Contacts, i)
		if contact != nil {
			list := GetClientsList(contact.Pid)
			if len(list) > 0 {
				sendMessage(conn, TMESS_STATUS, contact.Pid, "1")
			} else {
				sendMessage(conn, TMESS_STATUS, contact.Pid, "0")
			}
		}
	}
	return true
}

func processInfoContact(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришел запрос на информацию о контакте")
	if len(message.Messages) != 1 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	if curClient.Profile == nil {
		LogAdd(MessError, id+" не авторизован профиль")
		return false
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		p := GetContact(curClient.Profile.Contacts, i)
		if p != nil {
			list := GetClientsList(p.Pid)
			if len(list) != 0 {
				for _, peer := range list {
					sendMessage(peer.Conn, TMESS_INFO_CONTACT, curClient.Pid, p.Digest, p.Salt)
				}
			} else {
				LogAdd(MessError, id+" нет такого контакта в сети")
				if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
					sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageAbsentError))
				} else {
					sendMessage(conn, TMESS_NOTIFICATION, "Нет такого контакта в сети!") //todo удалить
				}
			}
		} else {
			LogAdd(MessError, id+" нет такого контакта в профиле")
			if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
				sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageAbsentError))
			} else {
				sendMessage(conn, TMESS_NOTIFICATION, "Нет такого контакта в профиле!") //todo удалить
			}
		}
	} else {
		LogAdd(MessError, id+" ошибка преобразования идентификатора")
		if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
			sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageAbsentError))
		} else {
			sendMessage(conn, TMESS_NOTIFICATION, "Ошибка преобразования идентификатора!") //todo удалить
		}
	}
	return true
}

func processInfoAnswer(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришел ответ на информацию о контакте")
	if len(message.Messages) < 1 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	list := GetClientsList(message.Messages[0])
	if len(list) > 0 {
		for _, peer := range list {
			if peer.Profile != nil {
				sendMessage(peer.Conn, TMESS_INFO_ANSWER, message.Messages...)
				LogAdd(MessInfo, id+" вернули ответ")
			} else {
				LogAdd(MessError, id+" деавторизованный профиль")
			}
		}

	} else {
		LogAdd(MessError, id+" нет такого контакта в сети")
		if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
			sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageAbsentError))
		} else {
			sendMessage(conn, TMESS_NOTIFICATION, "Нет такого контакта в сети!") //todo удалить
		}
	}
	return true
}

func processManage(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришел запрос на управление")
	if len(message.Messages) < 2 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	if curClient.Profile == nil {
		LogAdd(MessError, id+" не авторизован профиль")
		return false
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		p := GetContact(curClient.Profile.Contacts, i)
		if p != nil {
			list := GetClientsList(p.Pid)
			if len(list) > 0 {
				for _, peer := range list {
					var content []string
					content = append(content, curClient.Pid, p.Digest, p.Salt)
					content = append(content, message.Messages[1:]...)

					sendMessage(peer.Conn, TMESS_MANAGE, content...)
				}
			} else {
				LogAdd(MessError, id+" нет такого контакта в сети")
				if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
					sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageAbsentError))
				} else {
					sendMessage(conn, TMESS_NOTIFICATION, "Нет такого контакта в сети!") //todo удалить
				}
			}
		} else {
			LogAdd(MessError, id+" нет такого контакта в профиле")
			if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
				sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageAbsentError))
			} else {
				sendMessage(conn, TMESS_NOTIFICATION, "Нет такого контакта в профиле!") //todo удалить
			}
		}
	} else {
		LogAdd(MessError, id+" ошибка преобразования идентификатора")
		if curClient.GreaterVersionThan(MinimalVersionForStaticAlert) {
			sendMessage(conn, TMESS_STANDART_ALERT, fmt.Sprint(StaticMessageAbsentError))
		} else {
			sendMessage(conn, TMESS_NOTIFICATION, "Ошибка преобразования идентификатора!") //todo удалить
		}
	}
	return true
}

func processContactReverse(message Message, conn *net.Conn, curClient *Client, id string) bool {
	LogAdd(MessInfo, id+" пришел запрос на добавление в чужую учетку")

	if len(message.Messages) < 3 {
		LogAdd(MessError, id+" не правильное кол-во полей")
		return false
	}

	//Message[0] - login profile
	//Message[1] - digest
	//Message[2] - caption

	curProfile := GetProfile(message.Messages[0])
	if curProfile != nil {
		if GetSHA256(curProfile.Pass+curClient.Salt) == message.Messages[1] {
			i := GetNewId(curProfile.Contacts)

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
			AddContainedProfile(curClient.Pid, curProfile)

			//отправим всем авторизованным об изменениях
			for _, client := range GetAuthorizedClientList(curProfile.Email) {
				sendMessage(client.Conn, TMESS_CONTACT, fmt.Sprint(i), "node", c.Caption, c.Pid, "", "-1")
				sendMessage(client.Conn, TMESS_STATUS, fmt.Sprint(i), "1")
			}

			LogAdd(MessInfo, id+" операция с контактом выполнена")
			return true
		}
	}

	LogAdd(MessError, id+" не удалось добавить контакт в чужой профиль")
	return false
}

func processServers(message Message, conn *net.Conn, curClient *Client, id string) bool {
	//убедимся что версия клиента поддерживает соединения через агента
	if !curClient.GreaterVersionThan(MinimalVersionForNodes) {
		return false
	}

	LogAdd(MessInfo, id+" пришел запрос на информацию об агентах")

	if Options.Mode != ModeMaster {
		return false
	}

	nodesString := make([]string, 0)
	nodes.Range(func(key interface{}, value interface{}) bool {
		nodesString = append(nodesString, value.(*Node).Ip)
		return true
	})

	sendMessage(conn, TMESS_SERVERS, nodesString...)
	return true
}
