package common

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"os"
	"sync"
	"time"
)

const (
	WhitelabelName    = "reVisit server"
	WhitelabelVersion = "0.7"

	CodeLength     = 64 //длина code
	PasswordLength = 14

	VNCFileList = "vnc.json"

	MaxLengthIDLog        = 6 //длина идентификатора потока для логов
	MaxLengthIDNode       = 8
	LengthSalt            = 16
	LengthToken           = 32
	WebSessionTimeoutHour = 2
	PredefinedPass        = "0000"
	UriServiceIpIFY       = "https://api.ipify.org/"
	UriYandexMap          = "https://api.lbs.yandex.net/geolocation"
	ReqYandexMap          = `{"common": {"version": "1.0","api_key": "%s"},"ip": {"address_v4": "%s"}}`

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

	FileOptions = "options.json"

	ModeRegular = 0
	ModeMaster  = 1
	ModeNode    = 2

	//виды сообщений логов
	MessError  = 1
	MessInfo   = 2
	MessDetail = 3
	MessFull   = 4

	CountersFilename = "counters.json"
	LogName          = "log.txt"
	maxLogFileMb     = 50

	//константы ожидания
	WaitCount        = 50
	WaitIdle         = 500
	WaitAfterConnect = 250
	WaitHelperCycle  = 5
	WaitPing         = 10
	WaitIdleAgent    = 2
	WaitConnection   = 30
	WriteTimeout     = 13

	ProfilesFilename = "profiles.json"
)

var (
	//опции по-умолчанию
	Options = options{
		MainServerPort:                "65471",
		DataServerPort:                "65475",
		HttpServerPort:                "8090",
		HttpsServerPort:               "8091",
		HttpsCertPath:                 "cert.pem",
		HttpsKeyPath:                  "key.pem",
		SizeBuff:                      16000,
		AdminLogin:                    "admin",
		AdminPass:                     "admin",
		Mode:                          ModeRegular,
		MyCoordinates:                 [2]float64{0, 0},
		MasterServer:                  "data.rvisit.net",
		MasterPort:                    "65470",
		MasterPassword:                "master",
		LogLevel:                      log.DebugLevel,
		IpCoordinateCacheTimeoutHours: 48,
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

		CounterDayYearBytes       [366]uint64
		CounterDayYearConnections [366]uint64
		CounterDayYearClients     [366]uint64

		CounterMonthBytes       [12]uint64
		CounterMonthConnections [12]uint64
		CounterMonthClients     [12]uint64

		mutex sync.Mutex
	}
)

// сохраняемые опции
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

	LogLevel                      log.Level
	IpCoordinateCacheTimeoutHours int
}

// информация о внц и основные команды для управления им
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
		log.Errorf("не удалось сохранить настройки: %s", err.Error())
		return
	}

	f, err := os.Create(FileOptions + ".tmp")
	if err != nil {
		log.Errorf("не удалось сохранить настройки: %s", err.Error())
		return
	}

	n, err := f.Write(b)
	if n == len(b) && err == nil {
		err = f.Close()
		if err != nil {
			log.Errorf("ошибка при сохранении настроек: %s", err.Error())
		}

		err = os.Remove(FileOptions)
		if err != nil {
			log.Errorf("ошибка при сохранении настроек: %s", err.Error())
		}

		err = os.Rename(FileOptions+".tmp", FileOptions)
		if err != nil {
			log.Errorf("ошибка при сохранении настроек: %s", err.Error())
		}
	} else {
		log.Errorf("не удалось сохранить настройки: %s", err.Error())

		err = f.Close()
		if err != nil {
			log.Errorf("ошибка при сохранении настроек: %s", err.Error())
		}
	}
}

func LoadOptions() {
	b, err := os.ReadFile(FileOptions)
	if err != nil {
		log.Errorf("не получилось загрузить настройки: %s", err.Error())
		return
	}

	if err = json.Unmarshal(b, &Options); err != nil {
		log.Errorf("не получилось загрузить настройки: %s", err.Error())
	}
}
