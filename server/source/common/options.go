package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

const (
	//REVISIT_VERSION - версия сервера или ноды, пока не используется
	REVISIT_VERSION = "0.7"

	//общие константы
	CODE_LENGTH     = 64 //длина code
	PASSWORD_LENGTH = 14

	FILE_VNCLIST    = "vnc.list"

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

	FILE_OPTIONS = "options.cfg"

	REGULAR = 0
	MASTER  = 1
	NODE    = 2

	//виды сообщений логов
	MESS_ERROR  = 1
	MESS_INFO   = 2
	MESS_DETAIL = 3
	MESS_FULL   = 4

	FILE_COUNTERS = "counters.json"
	LOG_NAME      = "log.txt"
	LOG_SIZE      = 20 * 1024 * 1024 //20MB

	//константы ожидания
	WAIT_COUNT         = 50
	WAIT_IDLE          = 500
	WAIT_AFTER_CONNECT = 250
	WAIT_HELPER_CYCLE  = 5
	WAIT_PING          = 10
	WAIT_IDLE_AGENT    = 2
	WAIT_CONNECTION    = 30
)

var (
	//опции по-умолчанию
	Options = options{
		MainServerPort:  "65471",
		DataServerPort:  "65475",
		HttpServerPort:  "8090",
		HttpsServerPort: "8091",
		HttpsCertPath:   "cert.pem",
		HttpsKeyPath:    "key.pem",
		SizeBuff:        16000,
		AdminLogin:      "admin",
		AdminPass:       "admin",
		Mode:            REGULAR,
		MyCoordinates:   [2]float64{0, 0},
		FDebug:          true,
		MasterServer:    "data.rvisit.net",
		MasterPort:      "65470",
		MasterPassword:  "master",
	}

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
)

//сохраняемые опции
type options struct {
	//настройки smtp сервера
	ServerSMTP string
	PortSMTP   string
	LoginSMTP  string
	PassSMTP   string

	//реквизиты сервера
	MainServerPort string

	//реквизиты сервер
	DataServerPort string

	//реквизиты веб сервера
	HttpServerPort  string
	HttpsServerPort string
	HttpsCertPath   string
	HttpsKeyPath    string

	//размер буфера для операций с сокетами
	SizeBuff int

	//учетка для админ панели
	AdminLogin string
	AdminPass  string

	//режим работы экземпляра сервера
	Mode          int
	MyCoordinates [2]float64
	Hostname      string

	//мастер сервер, если он нужен
	MasterServer   string
	MasterPort     string
	MasterPassword string

	//ключ для отображения на карте точек клиентов
	YandexApiKeyMap string

	//актуальная версия клиента, используем при обновлении и на сайте
	Version string

	//очевидно что флаг для отладки
	FDebug bool
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

func SaveOptions() {
	b, err := json.Marshal(Options)
	if err == nil {
		f, err := os.Create(FILE_OPTIONS + ".tmp")
		if err == nil {
			n, err := f.Write(b)
			if n == len(b) && err == nil {
				f.Close()

				os.Remove(FILE_OPTIONS)
				os.Rename(FILE_OPTIONS+".tmp", FILE_OPTIONS)
			} else {
				f.Close()
				LogAdd(MESS_ERROR, "Не удалось сохранить настройки: "+fmt.Sprint(err))
			}
		} else {
			LogAdd(MESS_ERROR, "Не удалось сохранить настройки: "+fmt.Sprint(err))
		}
	} else {
		LogAdd(MESS_ERROR, "Не удалось сохранить настройки: "+fmt.Sprint(err))
	}
}

func LoadOptions() {
	f, err := os.Open(FILE_OPTIONS)
	defer f.Close()
	if err == nil {
		b, err := ioutil.ReadAll(f)
		if err == nil {
			err = json.Unmarshal(b, &Options)
			if err != nil {
				LogAdd(MESS_ERROR, "Не получилось загрузить настройки: "+fmt.Sprint(err))
			}
		} else {
			LogAdd(MESS_ERROR, "Не получилось загрузить настройки: "+fmt.Sprint(err))
		}
	} else {
		LogAdd(MESS_ERROR, "Не получилось загрузить настройки: "+fmt.Sprint(err))
	}
}
