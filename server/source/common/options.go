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
	//ReVisitVersion - версия сервера или ноды, пока не используется
	ReVisitVersion = "0.7"

	//общие константы
	CodeLength     = 64 //длина code
	PasswordLength = 14

	VNCFileList = "vnc.list"

	MaxLengthIDLog        = 6 //длина идентификатора потока для логов
	MaxLengthIDNode       = 8
	LengthSalt            = 16
	LengthToken           = 32
	WebSessionTimeoutHour = 2

	PredefinedPass        = "0000"
	UriServiceIpIFY       = "https://api.ipify.org/"
	UriYandexMap          = "http://api.lbs.yandex.net/geolocation"
	ReqYandexMap          = `{
                              "common": {
                                "version": "1.0",
                                "api_key": "%s"
                              },
                              "ip": {
                                "address_v4": "%s"
                              }
                             }`

	//константы ограничений
	MinimalVersionForNodes       = 0.97
	MinimalVersionForStaticAlert = 0.98

	//статические сообщения
	StaticMessageEmpty         = 0
	StaticMessageNetworkError  = 1
	StaticMessageProxyError    = 2
	StaticMessageAuthError     = 3
	StaticMessageVncError      = 4
	StaticMessageTimeoutError  = 5
	StaticMessageAbsentError   = 6
	StaticMessageTypeError     = 7
	StaticMessageAuthFail      = 8
	StaticMessageRegFail       = 9
	StaticMessageRegMail       = 10
	StaticMessageRegSuccessful = 11

	FileOptions = "options.cfg"

	ModeRegular = 0
	ModeMaster  = 1
	ModeNode    = 2

	//виды сообщений логов
	MessError  = 1
	MessInfo   = 2
	MessDetail = 3
	MessFull   = 4

	CountersFilename = "counters.json"
	LogFilename      = "log.txt"
	LogSize          = 20 * 1024 * 1024 //20MB

	//константы ожидания
	WaitCount        = 50
	WaitIdle         = 500
	WaitAfterConnect = 250
	WaitHelperCycle  = 5
	WaitPing         = 10
	WaitIdleAgent    = 2
	WaitConnection   = 30

	ProfilesFilename = "profiles.list"
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
		Mode:            ModeRegular,
		MyCoordinates:   [2]float64{0, 0},
		DebugFlag:       true,
		MasterServer:    "data.rvisit.net",
		MasterPort:      "65470",
		MasterPassword:  "master",
		LogLevel:        MessFull,
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

	DebugFlag bool //очевидно что флаг для отладки
	LogLevel  int  //максимальный уровень логов
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
	b, err := json.MarshalIndent(Options, "", "  ")
	if err != nil {
		LogAdd(MessError, "Не удалось сохранить настройки: "+fmt.Sprint(err))
		return
	}

	f, err := os.Create(FileOptions + ".tmp")
	if err != nil {
		LogAdd(MessError, "Не удалось сохранить настройки: "+fmt.Sprint(err))
		return
	}

	n, err := f.Write(b)
	if n == len(b) && err == nil {
		err = f.Close()
		if err != nil {
			LogAdd(MessError, "Ошибка при сохранении настроек: "+fmt.Sprint(err))
		}

		err = os.Remove(FileOptions)
		if err != nil {
			LogAdd(MessError, "Ошибка при сохранении настроек: "+fmt.Sprint(err))
		}

		err = os.Rename(FileOptions+".tmp", FileOptions)
		if err != nil {
			LogAdd(MessError, "Ошибка при сохранении настроек: "+fmt.Sprint(err))
		}
	} else {
		LogAdd(MessError, "Не удалось сохранить настройки: "+fmt.Sprint(err))

		err = f.Close()
		if err != nil {
			LogAdd(MessError, "Ошибка при сохранении настроек: "+fmt.Sprint(err))
		}
	}
}

func LoadOptions() {
	f, err := os.Open(FileOptions)
	if err != nil {
		LogAdd(MessError, "Не получилось загрузить настройки: "+fmt.Sprint(err))
		return
	}
	defer func() {
		err = f.Close()
		if err != nil {
			LogAdd(MessError, "Ошибка при загрузки настроек: "+fmt.Sprint(err))
		}
	}()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		LogAdd(MessError, "Не получилось загрузить настройки: "+fmt.Sprint(err))
		return
	}

	err = json.Unmarshal(b, &Options)
	if err != nil {
		LogAdd(MessError, "Не получилось загрузить настройки: "+fmt.Sprint(err))
	}
}
