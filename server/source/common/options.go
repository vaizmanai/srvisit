package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

const (
	FILE_OPTIONS = "options.cfg"

	REGULAR = 0
	MASTER  = 1
	NODE    = 2
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
