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
	TMessDeauth         = 0  //деаутентификация()
	TMessVersion        = 1  //запрос версии
	TMessAuth           = 2  //аутентификация(генерация pid)
	TMessLogin          = 3  //вход в профиль
	TMessNotification   = 4  //сообщение клиент
	TMessRequest        = 5  //запрос на подключение
	TMessConnect        = 6  //запрашиваем подключение у клиента
	TMessDisconnect     = 7  //сообщаем об отключении клиенту
	TMessReg            = 8  //регистрация профиля
	TMessContact        = 9  //создание, редактирование, удаление
	TMessContacts       = 10 //запрос списка контактов
	TMessLogout         = 11 //выход из профиля
	TMessConnectContact = 12 //запрос подключения к контакту из профиля
	TMessStatuses       = 13 //запрос всех статусов
	TMessStatus         = 14 //запрос статуса
	TMessInfoContact    = 15 //запрос информации о клиенте
	TMessInfoAnswer     = 16 //ответ на запрос информации
	TMessManage         = 17 //запрос на управление(перезагрузка, обновление, переустановка)
	TMessPing           = 18 //проверка состояния подключения
	TMessContactReverse = 19 //добавление себя в чужой профиль
	TMessServers        = 20 //отправляем список агентов, чтобы клиент выбрал тот что нужен, тут же отправляем изменения
	TMessStandardAlert  = 21 //стандартные сообщения, чтобы была возможность интернационально выводить их

	TMessAgentDeauth   = 0
	TMessAgentAuth     = 1
	TMessAgentAddCode  = 2
	TMessAgentDelCode  = 3
	TMessAgentAddBytes = 4
	TMessAgentNewConn  = 5

	TMessAgentPing = 18
)

var (
	//функции для обработки сообщений
	Processing = []processingMessage{
		{TMessDeauth, nil},
		{TMessVersion, processVersion},
		{TMessAuth, processAuth},
		{TMessLogin, processLogin},
		{TMessNotification, processNotification},
		{TMessRequest, processConnect},
		{TMessConnect, nil},
		{TMessDisconnect, processDisconnect},
		{TMessReg, processReg},
		{TMessContact, processContact},
		{TMessContacts, processContacts}, //10
		{TMessLogout, processLogout},
		{TMessConnectContact, processConnectContact},
		{TMessStatuses, processStatuses},
		{TMessStatus, processStatus},
		{TMessInfoContact, processInfoContact},
		{TMessInfoAnswer, processInfoAnswer},
		{TMessManage, processManage},
		{TMessPing, processPing},
		{TMessContactReverse, processContactReverse},
		{TMessServers, processServers}, //20
		{TMessStandardAlert, nil}}

	ProcessingAgent = []processingAgent{
		{TMessAgentDeauth, nil},
		{TMessAgentAuth, processAgentAuth},
		{TMessAgentAddCode, processAgentAddCode},
		{TMessAgentDelCode, processAgentDelCode},
		{TMessAgentAddBytes, processAgentAddBytes},
		{TMessAgentNewConn, processAgentNewConn},

		18: {TMessAgentPing, processAgentPing}}

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

//информация о ноде
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

func ping(conn *net.Conn) {
	success := true
	for success {
		time.Sleep(time.Second * WaitPing)
		success = sendMessage(conn, TMessPing)
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
	if err != nil {
		LogAdd(MessError, err.Error())
		return false
	}
	return sendRawBytes(conn, out)
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
				sendMessage(authClient.Conn, TMessStatus, CleanPid(client.Pid), "1")
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
				statuses = append(statuses, printMessage(TMessStatus, fmt.Sprint(CleanPid(first.Pid)), "1")...)
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
	err := (*conn).SetWriteDeadline(time.Now().Add(time.Second * WriteTimeout))
	if err != nil {
		return false
	}

	_, err = (*conn).Write(bytes)
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
