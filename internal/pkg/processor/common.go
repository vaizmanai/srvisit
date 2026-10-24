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
	"sync"
	"time"
)

const (
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
	//Processing функции для обработки сообщений
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
	master      *net.Conn
	masterMutex sync.RWMutex

	//messStaticText текстовая расшифровка статических сообщений
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
	arrayVnc   []common.VNC

	//только для отображения на карте используем
	myIp          = ""
	coordinates   [2]float64
	locationMutex sync.RWMutex
	vncMutex      sync.RWMutex
)

// Node информация о ноде
type Node struct {
	Id          string
	Name        string
	Ip          string
	Conn        *net.Conn
	coordinates [2]float64
}

// Message структура для сообщения
type Message struct {
	TMessage int
	Messages []string
}

// dConn double pointer
type dConn struct {
	client  *client.Client //кому транслируем
	server  *client.Client //кто транслирует
	pointer [2]*net.Conn
	node    *Node
	address string
	ready   chan struct{}
	closed  bool
	mutex   sync.RWMutex
}

// processingAgent обработчик для запросов агенту
type processingAgent struct {
	TMessage   int
	Processing func(message Message, conn *net.Conn, curNode *Node, id string)
}

// processingMessage обработчик для сообщений
type processingMessage struct {
	TMessage   int
	Processing func(message Message, conn *net.Conn, curClient *client.Client, id string) bool
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
		time.Sleep(time.Second * common.WaitPing)
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
	if conn == nil || *conn == nil {
		log.Errorf("нет сокета для отправки")
		return false
	}

	var mes Message
	mes.TMessage = TMessage
	mes.Messages = Messages

	out, err := json.Marshal(mes)
	if err != nil {
		log.Error(err.Error())
		return false
	}
	return sendRawBytes(conn, out)
}

func sendMessageToAllClients(TMessage int, Messages ...string) {
	for _, c := range client.GetAllClientsList() {
		if c != nil {
			sendMessage(c.Snapshot().Conn, TMessage, Messages...)
		}
	}
}

func HelperThread() {
	log.Infof("helperThread запустился")
	for {
		profile.SaveProfiles()
		common.CleanOldCacheCoordinates()
		common.SwiftCounter()
		common.RotateLogFiles()

		time.Sleep(time.Second * common.WaitHelperCycle)
	}
}

// addClientToProfile пробежимся по профилям и найдем, где есть контакты с нашим пид и добавим этот профиль нам
func addClientToProfile(c *client.Client) {
	for _, p := range profile.GetProfileList() {
		//если этот клиент есть в конкретном профиле
		p.Lock()
		contained := contact.GetContactByPid(p.Contacts, common.CleanPid(c.Snapshot().Pid)) != nil
		p.Unlock()
		if contained {
			client.AddContainedProfile(c.Pid, p)

			//отправим всем авторизованным в этот профиль обновление статуса
			for _, authClient := range client.GetAuthorizedClientList(p.Email) {
				sendMessage(authClient.Snapshot().Conn, TMessStatus, common.CleanPid(c.Snapshot().Pid), "1")
			}
		}
	}
}

func checkStatuses(curClient *client.Client, first *contact.Contact) {
	var statuses []byte
	for first != nil {
		if first.Type != contact.TypeFolder {
			list := client.GetClientsList(first.Pid)
			if list != nil && len(list) > 0 {
				//todo хз что делать, у нас может быть совсем не интересующий нас контакт онлайн из-за потенциальных дублей
				statuses = append(statuses, printMessage(TMessStatus, fmt.Sprint(common.CleanPid(first.Pid)), "1")...)
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
	if conn == nil || *conn == nil {
		return false
	}
	err := (*conn).SetWriteDeadline(time.Now().Add(time.Second * common.WriteTimeout))
	if err != nil {
		return false
	}

	_, err = writeAll(*conn, bytes)
	if err != nil {
		return false
	}
	return true
}

func UpdateMyIP() {
	ip := common.GetMyIpByExternalApi()
	position := common.GetOptions().MyCoordinates
	if position == [2]float64{} {
		position = common.GetCoordinatesByYandex(ip)
	}
	locationMutex.Lock()
	myIp, coordinates = ip, position
	locationMutex.Unlock()
}

func LoadVNCList() {
	var list []common.VNC
	if err := common.LoadFile(common.VNCFileList, &list); err != nil {
		log.Errorf("loading file: %s", err.Error())
		return
	}
	vncMutex.Lock()
	defer vncMutex.Unlock()
	arrayVnc = list
	defaultVnc = 0
}
