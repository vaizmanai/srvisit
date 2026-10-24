package processor

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/client"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
	"github.com/vaizmanai/srvisit/internal/pkg/contact"
	"github.com/vaizmanai/srvisit/internal/pkg/profile"
	"net"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func processVersion(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришла информация о версии", id)

	if len(message.Messages) != 1 {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}

	curClient.SetVersion(message.Messages[0])
	return true
}

func processAuth(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришла авторизация", id)

	if len(message.Messages) != 1 || curClient.Snapshot().Pid != "" {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}
	if len(message.Messages[0]) < 3 {
		time.Sleep(time.Millisecond * common.WaitIdle)
		sendMessage(conn, TMessDeauth)
		log.Errorf("%s слабый serial", id)
		return false
	}

	s := common.GetPid(message.Messages[0])
	log.Infof("%s сгенерировали pid", id)

	salt := common.RandomString(common.LengthSalt)
	token := common.RandomString(common.LengthToken)

	if sendMessage(conn, TMessAuth, s, salt, token) {
		curClient.SetIdentity(message.Messages[0], s, salt, token, conn)
		curClient.StoreClient()
		curClient.SetCoordinates([2]float64{0, 0})

		addClientToProfile(curClient)
		log.Infof("%s авторизация успешна", id)

		//получим координаты по ip
		go func() {
			h, _, err := net.SplitHostPort((*conn).RemoteAddr().String())
			if err == nil {
				curClient.SetCoordinates(common.GetCoordinatesByYandex(h))
			}
		}()
		return true
	}
	return false
}

func processNotification(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s уведомление пришло", id)

	if len(message.Messages) != 2 {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}

	//todo надо бы как-то защититься от спама
	list := client.GetClientsList(message.Messages[0])
	for _, peer := range list {
		sendMessage(peer.Snapshot().Conn, TMessNotification, message.Messages[1])
	}

	return true
}

func processConnect(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s обрабатываем запрос на подключение", id)

	if len(message.Messages) < 2 {
		log.Errorf("%s не правильное кол-во полей", id)
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

	list := client.GetClientsList(message.Messages[0])

	successfully := false
	passDigest := message.Messages[1]

	//отправим запрос на подключение всем, ответит только тот у кого пароль совпадет
	for _, peer := range list {
		code := common.RandomString(common.CodeLength)

		//убедимся что версия клиента поддерживает соединения через агента
		if !peer.GreaterVersionThan(common.MinimalVersionForNodes) {
			address = ""
		}
		connectPeers(code, curClient, peer, address)

		log.Infof("%s запрашиваем коммуникацию у %s для %s", id, (*peer.Conn).RemoteAddr(), code)
		if !sendMessage(peer.Snapshot().Conn, TMessConnect, passDigest, salt, code, "simple", "server", curClient.Pid, address) { //тот кто передает трансляцию
			disconnectPeers(code)
			log.Errorf("%s не смогли отправить запрос %s", id, code)
			if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
				sendMessage(curClient.Conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageNetworkError))
			}
		}

		successfully = true
	}

	if successfully {
		return true
	}

	log.Infof("%s нет такого пира", id)
	if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		sendMessage(curClient.Conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageAbsentError))
	} else {
		sendMessage(curClient.Conn, TMessNotification, "Нет такого пира") //todo удалить
	}

	return false
}

func processDisconnect(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришел запрос на отключение", id)
	if len(message.Messages) < 1 {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}

	code := message.Messages[0]
	if len(code) == 0 {
		return false
	}

	if len(message.Messages) > 1 {
		i, err := strconv.Atoi(message.Messages[1])
		if err == nil && i >= 0 && i < len(messStaticText) {
			log.Errorf("%s текст ошибки: %s", id, messStaticText[i])
			value, exists := channels.Load(code)
			if exists {
				peers := value.(*dConn)
				if peers.client != nil && peers.client.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
					sendMessage(peers.client.Snapshot().Conn, TMessStandardAlert, message.Messages[1])
				}
			}
		}
	}

	disconnectPeers(code)
	return true
}

func processPing(_ Message, _ *net.Conn, _ *client.Client, _ string) bool {
	//log.Infof("%s пришел пинг", id)
	return true
}

func processLogin(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришел запрос на авторизацию профиля", id)
	if len(message.Messages) != 2 {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}

	email := strings.ToLower(message.Messages[0])
	p := profile.GetProfile(email)
	if p != nil {
		if message.Messages[1] == common.GetSHA256(p.Password()+curClient.Salt) {
			log.Infof("%s авторизация профиля пройдена", id)

			sendMessage(conn, TMessLogin)
			if previous := curClient.SetProfile(p); previous != nil {
				client.DelAuthorizedClient(previous.Email, curClient)
			}
			client.AddAuthorizedClient(p.Email, curClient)
			processContacts(message, conn, curClient, id)
			return true
		}
	} else {
		log.Errorf("%s нет такой учетки", id)
	}

	log.Errorf("%s авторизация профиля не успешна", id)
	if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageAuthFail))
	} else {
		sendMessage(conn, TMessNotification, "Авторизация профиля провалилась!") //todo удалить
	}
	return true
}

func processReg(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришел запрос на регистрацию", id)
	if len(message.Messages) != 1 {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}

	//проверяем доступность учетки
	email := strings.ToLower(strings.TrimSpace(message.Messages[0]))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return false
	}
	smtpEnabled := common.GetOptions().ServerSMTP != ""
	if !smtpEnabled && profile.GetProfile(email) == nil {
		if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
			sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageRegMail))
		} else {
			sendMessage(conn, TMessNotification, "Регистрация недоступна: SMTP не настроен.")
		}
		return false
	}
	password := common.RandomString(common.PasswordLength)
	newProfile := profile.NewProfileWithPassword(email, password)
	if newProfile != nil {
		if smtpEnabled {
			if _, err := common.SendEmail(message.Messages[0], fmt.Sprintf("Information from %s", common.WhitelabelName), fmt.Sprintf("Your password is %s", newProfile.Pass)); err != nil {
				profile.DelProfile(newProfile.Email)
				log.Errorf("%s не удалось отправить письмо с паролем: %s", id, err.Error())
				if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
					sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageRegMail))
				} else {
					sendMessage(conn, TMessNotification, "Не удалось отправить письмо с паролем!") //todo удалить
				}
				return false
			}
		}

		sendMessage(conn, TMessReg, "success")
		if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
			sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageRegSuccessful))
		} else {
			sendMessage(conn, TMessNotification, "Учетная запись создана, Ваш пароль на почте!") //todo удалить
		}
		log.Infof("%s создали учетку", id)
	} else {
		//todo восстановление пароля

		log.Infof("%s такая учетка уже существует", id)
		if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
			sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageRegFail))
		} else {
			sendMessage(conn, TMessNotification, "Такая учетная запись уже существует!") //todo удалить
		}
	}
	return true
}

func processContact(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришел запрос на контакта", id)
	if len(message.Messages) != 6 {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}

	p := curClient.Profile
	if p == nil {
		log.Errorf("%s не авторизован профиль", id)
		return false
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		p.Lock()
		sendStatus := false

		if i == -1 {
			i = contact.GetNewId(p.Contacts)
		}

		if message.Messages[1] == "del" {
			p.Contacts = contact.DelContact(p.Contacts, i) //удаляем ссылки на контакт
		} else {
			c := contact.GetContact(p.Contacts, i)

			//если нет такого - создадим
			if c == nil {
				c = &contact.Contact{}
				if len(message.Messages[5]) == 0 { //если не указан родитель, то в корень
					c.Next = p.Contacts
					p.Contacts = c
				}
			}

			if len(message.Messages[5]) > 0 { //поменяем родителя
				parentID, parseErr := strconv.Atoi(message.Messages[5])
				if parseErr == nil && (parentID == i || contact.GetContact(c.Inner, parentID) != nil) {
					p.Unlock()
					return false
				}
				p.Contacts = contact.DelContact(p.Contacts, i) //удаляем ссылки на контакт

				parentId, err := strconv.Atoi(message.Messages[5]) //IndexParent ищем нового родителя
				if err == nil {
					parentContact := contact.GetContact(p.Contacts, parentId)
					if parentContact != nil {
						c.Next = parentContact.Inner
						parentContact.Inner = c
					} else {
						c.Next = p.Contacts
						p.Contacts = c
					}
				} else {
					c.Next = p.Contacts
					p.Contacts = c
				}
			}

			c.Id = i
			c.Type = contact.Type(message.Messages[1])
			c.Caption = message.Messages[2]
			c.Pid = message.Messages[3]
			if len(message.Messages[4]) > 0 {
				c.Digest = message.Messages[4]
				c.Salt = curClient.Salt
			}
			message.Messages[0] = fmt.Sprint(i)

			//если такой пид онлайн - добавить наш профиль туда
			list := client.GetClientsList(message.Messages[3])
			for _, peer := range list {
				client.AddContainedProfile(peer.Pid, p)
			}

			if len(c.Pid) > 0 {
				sendStatus = true
			}
		}
		p.Unlock()
		if sendStatus {
			processStatus(createMessage(TMessStatus, fmt.Sprint(i)), conn, curClient, id)
		}

		//отправим всем авторизованным об изменениях
		for _, authClient := range client.GetAuthorizedClientList(p.Email) {
			sendMessage(authClient.Snapshot().Conn, message.TMessage, message.Messages...)
		}

		log.Infof("%s операция с контактом выполнена", id)
		return true
	}
	log.Errorf("%s операция с контактом провалилась", id)
	return false
}

func processContacts(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришел запрос на обновления контактов", id)

	if curClient.Profile == nil {
		log.Errorf("%s профиль не авторизован", id)
		return false
	}

	//отправляем все контакты
	b, err := json.Marshal(curClient.Profile.Snapshot().Contacts)
	if err != nil {
		log.Errorf("%s не получилось отправить контакты: %s", id, err.Error())
		return false
	}

	enc := url.PathEscape(string(b))
	sendMessage(conn, TMessContacts, enc)
	log.Infof("%s отправили контакты", id)

	processStatuses(createMessage(TMessStatuses), conn, curClient, id)
	return true
}

func processLogout(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришел запрос на выход", id)

	if curClient.Profile == nil {
		log.Errorf("%s не авторизован профиль", id)
		return false
	}

	client.DelAuthorizedClient(curClient.Profile.Email, curClient)
	curClient.SetProfile(nil)
	return true
}

func processConnectContact(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришел запрос на подключение к контакту", id)
	if len(message.Messages) < 1 {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}

	p := curClient.Profile
	if p == nil {
		log.Errorf("%s не авторизован профиль", id)
		return false
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		c := contact.GetContact(p.Snapshot().Contacts, i)
		if c != nil {
			if len(message.Messages) > 1 {
				processConnect(createMessage(TMessConnect, c.Pid, c.Digest, c.Salt, message.Messages[1]), conn, curClient, id)
			} else {
				processConnect(createMessage(TMessConnect, c.Pid, c.Digest, c.Salt), conn, curClient, id)
			}
		} else {
			log.Errorf("%s нет такого контакта в профиле", id)
			if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
				sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageAbsentError))
			} else {
				sendMessage(conn, TMessNotification, "Нет такого контакта в профиле!") //todo удалить
			}
		}
	} else {
		log.Errorf("%s ошибка преобразования идентификатора", id)
		if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
			sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageAbsentError))
		} else {
			sendMessage(conn, TMessNotification, "Ошибка преобразования идентификатора!") //todo удалить
		}
	}
	return true
}

func processStatuses(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришел запрос на статусы профиля", id)
	if len(message.Messages) != 0 {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}

	if curClient.Profile == nil {
		log.Errorf("%s не авторизован профиль", id)
		return false
	}

	checkStatuses(curClient, curClient.Profile.Snapshot().Contacts)
	return true
}

func processStatus(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришел запрос на статус контакта", id)
	if len(message.Messages) != 1 {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}

	if curClient.Profile == nil {
		log.Errorf("%s не авторизован профиль", id)
		return false
	}

	if len(message.Messages[0]) == 0 {
		log.Errorf("%s пустой индекс", id)
		return false
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		c := contact.GetContact(curClient.Profile.Snapshot().Contacts, i)
		if c != nil {
			list := client.GetClientsList(c.Pid)
			if len(list) > 0 {
				sendMessage(conn, TMessStatus, c.Pid, "1")
			} else {
				sendMessage(conn, TMessStatus, c.Pid, "0")
			}
		}
	}
	return true
}

func processInfoContact(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришел запрос на информацию о контакте", id)
	if len(message.Messages) != 1 {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}

	if curClient.Profile == nil {
		log.Errorf("%s не авторизован профиль", id)
		return false
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		c := contact.GetContact(curClient.Profile.Snapshot().Contacts, i)
		if c != nil {
			list := client.GetClientsList(c.Pid)
			if len(list) != 0 {
				for _, peer := range list {
					sendMessage(peer.Snapshot().Conn, TMessInfoContact, curClient.Pid, c.Digest, c.Salt)
				}
			} else {
				log.Errorf("%s нет такого контакта в сети", id)
				if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
					sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageAbsentError))
				} else {
					sendMessage(conn, TMessNotification, "Нет такого контакта в сети!") //todo удалить
				}
			}
		} else {
			log.Errorf("%s нет такого контакта в профиле", id)
			if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
				sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageAbsentError))
			} else {
				sendMessage(conn, TMessNotification, "Нет такого контакта в профиле!") //todo удалить
			}
		}
	} else {
		log.Errorf("%s ошибка преобразования идентификатора", id)
		if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
			sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageAbsentError))
		} else {
			sendMessage(conn, TMessNotification, "Ошибка преобразования идентификатора!") //todo удалить
		}
	}
	return true
}

func processInfoAnswer(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришел ответ на информацию о контакте", id)
	if len(message.Messages) < 1 {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}

	list := client.GetClientsList(message.Messages[0])
	if len(list) > 0 {
		for _, peer := range list {
			state := peer.Snapshot()
			if state.Profile != nil {
				sendMessage(state.Conn, TMessInfoAnswer, message.Messages...)
				log.Infof("%s вернули ответ", id)
			} else {
				log.Errorf("%s деавторизованный профиль", id)
			}
		}

	} else {
		log.Errorf("%s нет такого контакта в сети", id)
		if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
			sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageAbsentError))
		} else {
			sendMessage(conn, TMessNotification, "Нет такого контакта в сети!") //todo удалить
		}
	}
	return true
}

func processManage(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришел запрос на управление", id)
	if len(message.Messages) < 2 {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}

	if curClient.Profile == nil {
		log.Errorf("%s не авторизован профиль", id)
		return false
	}

	i, err := strconv.Atoi(message.Messages[0])
	if err == nil {
		c := contact.GetContact(curClient.Profile.Snapshot().Contacts, i)
		if c != nil {
			list := client.GetClientsList(c.Pid)
			if len(list) > 0 {
				for _, peer := range list {
					var content []string
					content = append(content, curClient.Pid, c.Digest, c.Salt)
					content = append(content, message.Messages[1:]...)

					sendMessage(peer.Snapshot().Conn, TMessManage, content...)
				}
			} else {
				log.Errorf("%s нет такого контакта в сети", id)
				if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
					sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageAbsentError))
				} else {
					sendMessage(conn, TMessNotification, "Нет такого контакта в сети!") //todo удалить
				}
			}
		} else {
			log.Errorf("%s нет такого контакта в профиле", id)
			if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
				sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageAbsentError))
			} else {
				sendMessage(conn, TMessNotification, "Нет такого контакта в профиле!") //todo удалить
			}
		}
	} else {
		log.Errorf("%s ошибка преобразования идентификатора", id)
		if curClient.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
			sendMessage(conn, TMessStandardAlert, fmt.Sprint(common.StaticMessageAbsentError))
		} else {
			sendMessage(conn, TMessNotification, "Ошибка преобразования идентификатора!") //todo удалить
		}
	}
	return true
}

func processContactReverse(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	log.Infof("%s пришел запрос на добавление в чужую учетку", id)

	if len(message.Messages) < 3 {
		log.Errorf("%s не правильное кол-во полей", id)
		return false
	}

	//Message[0] - login profile
	//Message[1] - digest
	//Message[2] - caption

	curProfile := profile.GetProfile(message.Messages[0])
	if curProfile != nil {
		curProfile.Lock()
		if common.GetSHA256(curProfile.Pass+curClient.Salt) == message.Messages[1] {
			i := contact.GetNewId(curProfile.Contacts)

			c := &contact.Contact{}
			c.Next = curProfile.Contacts //добавляем пока только в корень
			curProfile.Contacts = c

			c.Id = i
			c.Type = "node"
			c.Caption = message.Messages[2]
			c.Pid = curClient.Pid
			c.Digest = message.Messages[1]
			c.Salt = curClient.Salt
			caption, pid := c.Caption, c.Pid
			curProfile.Unlock()

			//добавим этот профиль к авторизованному списку
			client.AddContainedProfile(curClient.Pid, curProfile)

			//отправим всем авторизованным об изменениях
			for _, client := range client.GetAuthorizedClientList(curProfile.Email) {
				sendMessage(client.Snapshot().Conn, TMessContact, fmt.Sprint(i), "node", caption, pid, "", "-1")
				sendMessage(client.Snapshot().Conn, TMessStatus, fmt.Sprint(i), "1")
			}

			log.Infof("%s операция с контактом выполнена", id)
			return true
		}
		curProfile.Unlock()
	}

	log.Errorf("%s не удалось добавить контакт в чужой профиль", id)
	return false
}

func processServers(message Message, conn *net.Conn, curClient *client.Client, id string) bool {
	//убедимся что версия клиента поддерживает соединения через агента
	if !curClient.GreaterVersionThan(common.MinimalVersionForNodes) {
		return false
	}

	log.Infof("%s пришел запрос на информацию об агентах", id)

	if common.GetOptions().Mode != common.ModeMaster {
		return false
	}

	nodesString := make([]string, 0)
	nodes.Range(func(key interface{}, value interface{}) bool {
		nodesString = append(nodesString, value.(*Node).Ip)
		return true
	})

	sendMessage(conn, TMessServers, nodesString...)
	return true
}
