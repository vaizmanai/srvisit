package service

import (
	. "../common"
	. "../component/client"
	. "../component/contact"
	. "../component/profile"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
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
	Processing func(message Message, conn *net.Conn, curClient *Client, id string) bool
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

func sendMessageToAllClients(TMessage int, Messages ...string) {
	for _, client := range GetAllClientsList() {
		if client != nil {
			sendMessage((*client).Conn, TMessage, Messages...)
		}
	}
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

//пробежимся по профилям, найдем где есть контакты с нашим пид и добавим этот профиль нам
func addClientToProfile(client *Client) {
	for _, profile := range GetProfileList() {
		//если этот клиент есть в конкретном профиле
		if GetContactByPid(profile.Contacts, CleanPid(client.Pid)) != nil {
			AddContainedProfile(client.Pid, profile)

			//отправим всем авторизованным в этот профиль обновление статуса
			for _, authClient := range GetAuthorizedClientList(profile.Email) {
				sendMessage(authClient.Conn, TMESS_STATUS, CleanPid(client.Pid), "1")
			}
		}
	}
}

func checkStatuses(curClient *Client, first *Contact) {
	var statuses []byte
	for first != nil {
		if first.Type != "fold" {
			list := GetClientsList(first.Pid)
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
