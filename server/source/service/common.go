package service

import (
	. "../common"
	. "../component/contact"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	//виды сообщений
	TMESS_DEAUTH          = 0  //деаутентификация()
	TMESS_VERSION         = 1  //запрос версии
	TMESS_AUTH            = 2  //аутентификация(генерация pid)
	TMESS_LOGIN           = 3  //вход в профиль
	TMESS_NOTIFICATION    = 4  //сообщение клиент
	TMESS_REQUEST         = 5  //запрос на подключение
	TMESS_CONNECT         = 6  //запрашиваем подключение у клиента
	TMESS_DISCONNECT      = 7  //сообщаем об отключении клиенту
	TMESS_REG             = 8  //регистрация профиля
	TMESS_CONTACT         = 9  //создание, редактирование, удаление
	TMESS_CONTACTS        = 10 //запрос списка контактов
	TMESS_LOGOUT          = 11 //выход из профиля
	TMESS_CONNECT_CONTACT = 12 //запрос подключения к конакту из профиля
	TMESS_STATUSES        = 13 //запрос всех статусов
	TMESS_STATUS          = 14 //запрос статуса
	TMESS_INFO_CONTACT    = 15 //запрос информации о клиенте
	TMESS_INFO_ANSWER     = 16 //ответ на запрос информации
	TMESS_MANAGE          = 17 //запрос на управление(перезагрузка, обновление, переустановка)
	TMESS_PING            = 18 //проверка состояния подключения
	TMESS_CONTACT_REVERSE = 19 //добавление себя в чужой профиль
	TMESS_SERVERS         = 20 //отправляем список агентов, чтобы клиент выбрал тот что нужен, тут же отправляем изменения
	TMESS_STANDART_ALERT  = 21 //стандартные сообщения, чтобы была возможность интернационально выводить их

	TMESS_AGENT_DEAUTH    = 0
	TMESS_AGENT_AUTH      = 1
	TMESS_AGENT_ADD_CODE  = 2
	TMESS_AGENT_DEL_CODE  = 3
	TMESS_AGENT_ADD_BYTES = 4
	TMESS_AGENT_NEW_CONN  = 5

	TMESS_AGENT_PING = 18

	FILE_PROFILES = "profiles.list"
)

var (
	//функции для обработки сообщений
	Processing = []processingMessage{
		{TMESS_DEAUTH, nil},
		{TMESS_VERSION, processVersion},
		{TMESS_AUTH, processAuth},
		{TMESS_LOGIN, processLogin},
		{TMESS_NOTIFICATION, processNotification},
		{TMESS_REQUEST, processConnect},
		{TMESS_CONNECT, nil},
		{TMESS_DISCONNECT, processDisconnect},
		{TMESS_REG, processReg},
		{TMESS_CONTACT, processContact},
		{TMESS_CONTACTS, processContacts}, //10
		{TMESS_LOGOUT, processLogout},
		{TMESS_CONNECT_CONTACT, processConnectContact},
		{TMESS_STATUSES, processStatuses},
		{TMESS_STATUS, processStatus},
		{TMESS_INFO_CONTACT, processInfoContact},
		{TMESS_INFO_ANSWER, processInfoAnswer},
		{TMESS_MANAGE, processManage},
		{TMESS_PING, processPing},
		{TMESS_CONTACT_REVERSE, processContactReverse},
		{TMESS_SERVERS, processServers}, //20
		{TMESS_STANDART_ALERT, nil}}

	ProcessingAgent = []processingAgent{
		{TMESS_AGENT_DEAUTH, nil},
		{TMESS_AGENT_AUTH, processAgentAuth},
		{TMESS_AGENT_ADD_CODE, processAgentAddCode},
		{TMESS_AGENT_DEL_CODE, processAgentDelCode},
		{TMESS_AGENT_ADD_BYTES, processAgentAddBytes},
		{TMESS_AGENT_NEW_CONN, processAgentNewConn},

		18: {TMESS_AGENT_PING, processAgentPing}}

	//карта подключенных клиентов
	//clients 		sync.Map
	clients     map[string][]*Client
	clientMutex sync.Mutex

	//карта учеток
	profiles sync.Map

	//карта каналов для передачи данных
	channels sync.Map

	//агенты обработки данных
	nodes sync.Map

	//сокет до мастера
	master *net.Conn

	//текстовая расшифровка статических сообщений
	messStaticText = []string{
		"пустое сообщение",
		"ошибка сети",
		"ошибка прокси",
		"ошибка авторизации",
		"ошибка VNC",
		"ошибка времени ожидания",
		"отсутствует пир",
		"не правильный тип подключения"}

	//список доступных vnc клиентов и выбранный по-умолчанию
	defaultVnc = 0
	arrayVnc   []VNC

	//только для отображения на карте используем
	myIp        = ""
	coordinates [2]float64
)

//тип для клиента
type Client struct {
	Serial  string
	Pid     string
	Pass    string
	Version string
	Salt    string //for password
	Profile *Profile
	Token   string //for web auth

	Conn *net.Conn
	Code string //for connection

	coordinates [2]float64
	profiles    sync.Map //профили которые содержат этого клиента в контактах(используем для отправки им информации о своем статусе)
}

//тип для профиля
type Profile struct {
	Email string
	Pass  string

	Contacts *Contact
	mutex    sync.Mutex

	clients sync.Map //клиенты которые авторизовались в этот профиль(используем для отправки им информации о статусе или изменений контактов)

	//всякая информация
	Capt string
	Tel  string
	Logo string
}

//информацияя о ноде
type Node struct {
	Id          string
	Name        string
	Ip          string
	Conn        *net.Conn
	coordinates [2]float64
}

//тип для сообщения
type Message struct {
	TMessage int
	Messages []string
}

//double pointer
type dConn struct {
	client  *Client //кто запросил трансляцию
	server  *Client //кто транслирует
	pointer [2]*net.Conn
	flag    [2]bool
	node    *Node
	address string
	mutex   sync.Mutex
}

//обработчик для запросов агенту
type processingAgent struct {
	TMessage   int
	Processing func(message Message, conn *net.Conn, curNode *Node, id string)
}

//обработчик для сообщений
type processingMessage struct {
	TMessage   int
	Processing func(message Message, conn *net.Conn, curClient *Client, id string)
}

func init() {
	clients = make(map[string][]*Client, 0)
}

func createMessage(TMessage int, Messages ...string) Message {
	var mes Message
	mes.TMessage = TMessage
	mes.Messages = Messages
	return mes
}

func Ping(conn *net.Conn) {
	success := true
	for success {
		time.Sleep(time.Second * WaitPing)
		success = sendMessage(conn, TMESS_PING)
	}
}

func printMessage(TMessage int, Messages ...string) []byte {
	var mes Message
	mes.TMessage = TMessage
	mes.Messages = Messages

	out, err := json.Marshal(mes)
	if err != nil {
		return []byte{}
	}

	return out
}

func sendMessage(conn *net.Conn, TMessage int, Messages ...string) bool {
	if conn == nil {
		LogAdd(MessError, "нет сокета для отправки")
		return false
	}

	var mes Message
	mes.TMessage = TMessage
	mes.Messages = Messages

	out, err := json.Marshal(mes)
	if err == nil && conn != nil {
		_, err = (*conn).Write(out)
		if err == nil {
			return true
		}
	}
	return false
}

func sendMessageToClients(TMessage int, Messages ...string) {
	for _, list := range clients {
		for _, client := range list {
			if client != nil {
				sendMessage((*client).Conn, TMessage, Messages...)
			}
		}
	}
}

func (client *Client) storeClient() {
	pid := CleanPid(client.Pid)
	clientMutex.Lock()

	list := clients[pid]
	if list == nil {
		list = make([]*Client, 0)
	}

	UpdateCounterClient(true)
	list = append(list, client)
	clients[pid] = list

	clientMutex.Unlock()
}

func (client *Client) removeClient() {
	pid := CleanPid(client.Pid)
	clientMutex.Lock()

	list := clients[pid]
	if list != nil {
		for i := 0; i < len(list); {
			if list[i] == client {
				if len(list) == 1 {
					UpdateCounterClient(false)
					list = make([]*Client, 0)
					break
				}
				UpdateCounterClient(false)
				list[i] = list[len(list)-1]
				list = list[:len(list)-1]
				continue
			}
			i++
		}
	}
	clients[pid] = list

	clientMutex.Unlock()
}

func HelperThread() {
	LogAdd(MessInfo, "helperThread запустился")
	for true {
		SaveProfiles()
		SwiftCounter()

		time.Sleep(time.Second * WaitHelperCycle)
	}
	LogAdd(MessInfo, "helperThread закончил работу")
}

func greaterVersionThan(client *Client, version float64) bool {

	peerVersion, err := strconv.ParseFloat(client.Version, 64)
	if err != nil || peerVersion < version {
		return false
	}

	return true
}

//пробежимся по профилям, найдем где есть контакты с нашим пид и добавим этот профиль нам
func addClientToProfile(client *Client) {
	profiles.Range(func(key interface{}, value interface{}) bool {
		profile := value.(*Profile)
		if addClientToContacts(profile.Contacts, client, profile) {
			//если мы есть хоть в одном конакте этого профиля, пробежимся по ним и отправим свой статус
			profile.clients.Range(func(key interface{}, value interface{}) bool {
				curClient := value.(*Client)
				sendMessage(curClient.Conn, TMESS_STATUS, CleanPid(client.Pid), "1")
				return true
			})
		}
		return true
	})
}

//пробежимся по всем контактам и если есть совпадение, то добавим ссылку на профиль этому клиенту
func addClientToContacts(contact *Contact, client *Client, profile *Profile) bool {
	res := false

	for contact != nil {
		if CleanPid(contact.Pid) == CleanPid(client.Pid) {
			client.profiles.Store(profile.Email, profile)
			res = true
		}

		if contact.Inner != nil {
			innerResult := addClientToContacts(contact.Inner, client, profile)
			if innerResult {
				res = true
			}
		}

		contact = contact.Next
	}

	return res
}

func checkStatuses(curClient *Client, first *Contact) {
	var statuses []byte
	for first != nil {
		if first.Type != "fold" {
			list := clients[CleanPid(first.Pid)]
			if list != nil && len(list) > 0 {
				//todo хз что делать, у нас может быть совсем не интересующий нас контакт онлайн из-за потенциальных дублей
				statuses = append(statuses, printMessage(TMESS_STATUS, fmt.Sprint(CleanPid(first.Pid)), "1")...)
			}
		}

		if first.Inner != nil {
			checkStatuses(curClient, first.Inner)
		}
		first = first.Next
	}
	//отправим статусы разом для этого уровня вложенности
	sendRawBytes(curClient.Conn, statuses)
}

func sendRawBytes(conn *net.Conn, bytes []byte) bool {
	_, err := (*conn).Write(bytes)
	if err != nil {
		return false
	}
	return true
}

func UpdateMyIP() {
	myIp = GetMyIpByExternalApi()
	if Options.MyCoordinates == [2]float64{0, 0} { //options.MyCoordinates[0] == 0 && options.MyCoordinates[1] == 0 {
		coordinates = GetCoordinatesByYandex(myIp)
	} else {
		coordinates = Options.MyCoordinates
	}
}

func SaveProfiles() {
	var list []*Profile

	profiles.Range(func(key interface{}, value interface{}) bool {
		profile := value.(*Profile)
		list = append(list, profile)
		return true
	})

	b, err := json.Marshal(list)
	if err == nil {
		f, err := os.Create(FILE_PROFILES + ".tmp")
		if err == nil {
			n, err := f.Write(b)
			if n == len(b) && err == nil {
				f.Close()

				os.Remove(FILE_PROFILES)
				os.Rename(FILE_PROFILES+".tmp", FILE_PROFILES)
			} else {
				f.Close()
				LogAdd(MessError, "Не удалось сохранить профили: "+fmt.Sprint(err))
			}
		} else {
			LogAdd(MessError, "Не удалось сохранить профили: "+fmt.Sprint(err))
		}
	} else {
		LogAdd(MessError, "Не удалось сохранить профили: "+fmt.Sprint(err))
	}
}

func LoadProfiles() {
	var list []Profile

	f, err := os.Open(FILE_PROFILES)
	defer f.Close()
	if err == nil {
		b, err := ioutil.ReadAll(f)
		if err == nil {
			err = json.Unmarshal(b, &list)
			if err == nil {
				for i := 0; i < len(list); i++ {
					profiles.Store(list[i].Email, &list[i])
				}
			} else {
				LogAdd(MessError, "Не получилось загрузить профили: "+fmt.Sprint(err))
			}
		} else {
			LogAdd(MessError, "Не получилось загрузить профили: "+fmt.Sprint(err))
		}
	} else {
		LogAdd(MessError, "Не получилось загрузить профили: "+fmt.Sprint(err))
	}
}

func LoadVNCList() {
	f, err := os.Open(VNCFileList)
	defer f.Close()
	if err == nil {
		b, err := ioutil.ReadAll(f)
		if err == nil {
			err = json.Unmarshal(b, &arrayVnc)
			if err == nil {
				defaultVnc = 0
			} else {
				LogAdd(MessError, "Не получилось загрузить список VNC: "+fmt.Sprint(err))
			}
		} else {
			LogAdd(MessError, "Не получилось загрузить список VNC: "+fmt.Sprint(err))
		}
	} else {
		LogAdd(MessError, "Не получилось загрузить список VNC: "+fmt.Sprint(err))
	}
}
