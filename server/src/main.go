package main

import (
	"../src/common"
	"../src/services"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"time"
)

func main() {
	for _, x := range os.Args {
		if strings.Contains(x, "node") {
			common.Options.Mode = common.NODE
		} else if strings.Contains(x, "master") {
			common.Options.Mode = common.MASTER
		}
	}

	common.LogAdd(common.MESS_INFO, "Запускается сервер reVisit версии "+common.REVISIT_VERSION)

	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())
	clients = make(map[string][]*Client, 0)

	common.LoadOptions()

	if common.Options.Mode != common.NODE {
		common.LoadVNCList()
		common.LoadCounters()
		common.LoadProfiles()

		go common.HelperThread() //используем для периодических действий(сохранения и т.п.)
		go services.HttpServer()   //обработка веб запросов
		go services.MainServer()   //обработка основных команд от клиентов и агентов
	}

	myIp = common.GetMyIpByExternalApi()
	if common.Options.MyCoordinates == [2]float64{0, 0} { //options.MyCoordinates[0] == 0 && options.MyCoordinates[1] == 0 {
		coordinates = common.GetCoordinatesByYandex(myIp)
	} else {
		coordinates = options.MyCoordinates
	}

	if common.Options.Mode != common.MASTER {
		go services.DataServer() //обработка потоков данных от клиентов
	}

	if common.Options.Mode == common.MASTER {
		go services.MasterServer() //общаемся с агентами
	}

	if common.Options.Mode == common.NODE {
		go services.NodeClient() //клинет подключающийся к мастеру
	}

	var r string
	for r != "quit" {
		fmt.Scanln(&r)
		time.Sleep(time.Millisecond * common.WAIT_IDLE) //если запустить без консоли, то здесь цикл со 100% загрузкой процессора
	}

	common.LogAdd(common.MESS_INFO, "Завершили работу")
}
