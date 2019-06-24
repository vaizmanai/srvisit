package main

import (
	"net"
	"sync"
	"time"
)

const (
	//REVISIT_VERSION - версия сервера или ноды, пока не используется
	REVISIT_VERSION = "0.7"

	//общие константы
	CODE_LENGTH      = 64 //длина code
	PASSWORD_LENGTH  = 14
	FILE_PROFILES    = "profiles.list"
	FILE_COUNTERS    = "counters.json"
	FILE_VNCLIST     = "vnc.list"

	MAX_LEN_ID_LOG   = 6
	MAX_LEN_ID_NODE  = 8
	LEN_SALT         = 16
	LEN_TOKEN        = 32
	WEB_TIMEOUT_HOUR = 2
	PREDEFINED_PASS  = "0000"
	URI_IPIFY_API    = "https://api.ipify.org/"
	URI_YANDEX_MAP   = "http://api.lbs.yandex.net/geolocation"
	REQ_YANDEX_MAP   = `{
							"common":{
								"version": "1.0",
								"api_key": "%s"
							},
							"ip":{
								"address_v4":"%s"
							}
						}`

	//константы ограничений
	MIN_VERSION_FOR_NODES        = 0.97
	MIN_VERSION_FOR_STATIC_ALERT = 0.98

	//статические сообщения
	STATIC_MESSAGE_EMPTY          = 0
	STATIC_MESSAGE_NETWORK_ERROR  = 1
	STATIC_MESSAGE_PROXY_ERROR    = 2
	STATIC_MESSAGE_AUTH_ERROR     = 3
	STATIC_MESSAGE_VNC_ERROR      = 4
	STATIC_MESSAGE_TIMEOUT_ERROR  = 5
	STATIC_MESSAGE_ABSENT_ERROR   = 6
	STATIC_MESSAGE_TYPE_ERROR     = 7
	STATIC_MESSAGE_AUTH_FAIL      = 8
	STATIC_MESSAGE_REG_FAIL       = 9
	STATIC_MESSAGE_REG_MAIL       = 10
	STATIC_MESSAGE_REG_SUCCESSFUL = 11

	//константы ожидания
	WAIT_COUNT         = 50
	WAIT_IDLE          = 500
	WAIT_AFTER_CONNECT = 250
	WAIT_HELPER_CYCLE  = 5
	WAIT_PING          = 10
	WAIT_IDLE_AGENT    = 2
	WAIT_CONNECTION    = 30



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

	//считаем всякую бесполезную информацию или нет
	counterData struct {
		currentPos time.Time

		CounterBytes       [24]uint64
		CounterConnections [24]uint64
		CounterClients     [24]uint64

		CounterDayWeekBytes       [7]uint64
		CounterDayWeekConnections [7]uint64
		CounterDayWeekClients     [7]uint64

		CounterDayBytes       [31]uint64
		CounterDayConnections [31]uint64
		CounterDayClients     [31]uint64

		CounterDayYearBytes       [365]uint64
		CounterDayYearConnections [365]uint64
		CounterDayYearClients     [365]uint64

		CounterMonthBytes       [12]uint64
		CounterMonthConnections [12]uint64
		CounterMonthClients     [12]uint64

		mutex sync.Mutex
	}

	//меню веб интерфейса админки
	menuAdmin = []itemMenu{
		{"Логи", "/admin/logs"},
		{"Настройки", "/admin/options"},
		{"Ресурсы", "/admin/resources"},
		{"Статистика", "/admin/statistics"},
		{"reVisit", "/resource/reVisit.exe"}}

	//меню веб интерфейса профиля
	menuProfile = []itemMenu{
		{"Профиль", "/profile/my"},
		{"reVisit", "/resource/reVisit.exe"}}

	//карта подключенных клиентов
	//clients 		sync.Map
	clients     map[string][]*Client
	clientMutex sync.Mutex

	//карта каналов для передачи данных
	channels sync.Map

	//карта учеток
	profiles sync.Map

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

	//функции для обработки сообщений
	processing = []ProcessingMessage{
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

	processingAgent = []ProcessingAgent{
		{TMESS_AGENT_DEAUTH, nil},
		{TMESS_AGENT_AUTH, processAgentAuth},
		{TMESS_AGENT_ADD_CODE, processAgentAddCode},
		{TMESS_AGENT_DEL_CODE, processAgentDelCode},
		{TMESS_AGENT_ADD_BYTES, processAgentAddBytes},
		{TMESS_AGENT_NEW_CONN, processAgentNewConn},

		18: {TMESS_AGENT_PING, processAgentPing}}

	//список доступных vnc клиентов и выбранный по-умолчанию
	defaultVnc = 0
	arrayVnc   []VNC

	//только для отображения на карте используем
	myIp        = ""
	coordinates [2]float64
)

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

//информацияя о ноде
type Node struct {
	Id          string
	Name        string
	Ip          string
	Conn        *net.Conn
	coordinates [2]float64
}

//обработчик для запросов агенту
type ProcessingAgent struct {
	TMessage   int
	Processing func(message Message, conn *net.Conn, curNode *Node, id string)
}

//обработчик для сообщений
type ProcessingMessage struct {
	TMessage   int
	Processing func(message Message, conn *net.Conn, curClient *Client, id string)
}

//тип для сообщения
type Message struct {
	TMessage int
	Messages []string
}



//информация о внц и основные команды для управления им
type VNC struct {
	FileServer string
	FileClient string

	//это команды используем для старта под админскими правами(обычно это создание сервиса)
	CmdStartServer   string
	CmdStopServer    string
	CmdInstallServer string
	CmdRemoveServer  string
	CmdConfigServer  string
	CmdManageServer  string

	//это комнады используем для старта без админских прав
	CmdStartServerUser   string
	CmdStopServerUser    string
	CmdInstallServerUser string
	CmdRemoveServerUser  string
	CmdConfigServerUser  string
	CmdManageServerUser  string

	//комнды для vnc клиента
	CmdStartClient   string
	CmdStopClient    string
	CmdInstallClient string
	CmdRemoveClient  string
	CmdConfigClient  string
	CmdManageClient  string

	PortServerVNC string
	Link          string
	Name          string
	Version       string
	Description   string
}

//меню для веба
type itemMenu struct {
	Capt string
	Link string
}

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

//тип для контакта
type Contact struct {
	Id      int
	Caption string
	Type    string //cont - контакт, fold - папка
	Pid     string
	Digest  string //но тут digest
	Salt    string

	Inner *Contact
	Next  *Contact
}

type YandexResp struct {
	Position YandexBody `json:"position"`
}

type YandexBody struct {
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
	Altitude          float64 `json:"altitude"`
	Precision         float32 `json:"precision"`
	AltitudePrecision float32 `json:"altitude_precision"`
	Type              string  `json:"type"`
}

type WebClientStatistic struct {
	Latitude  float64
	Longitude float64
	Pid       string
	Ip        string
	Note      string
}

type WebConnectionStatistic struct {
	Client1 WebClientStatistic
	Client2 WebClientStatistic

	Node WebClientStatistic
}
