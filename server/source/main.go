package main

import (
	. "./common"
	"./service"
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
			Options.Mode = NODE
		} else if strings.Contains(x, "master") {
			Options.Mode = MASTER
		}
	}

	LogAdd(MESS_INFO, "Запускается сервер reVisit версии "+REVISIT_VERSION)

	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	LoadOptions()

	if Options.Mode != NODE {
		service.LoadVNCList()
		LoadCounters()
		service.LoadProfiles()

		go service.HelperThread() //используем для периодических действий(сохранения и т.п.)
		go service.HttpServer()   //обработка веб запросов
		go service.MainServer()   //обработка основных команд от клиентов и агентов
	}

	service.UpdateMyIP()

	if Options.Mode != MASTER {
		go service.DataServer() //обработка потоков данных от клиентов
	}

	if Options.Mode == MASTER {
		go service.MasterServer() //общаемся с агентами
	}

	if Options.Mode == NODE {
		go service.NodeClient() //клинет подключающийся к мастеру
	}

	var r string
	for r != "quit" {
		fmt.Scanln(&r)
		time.Sleep(time.Millisecond * WAIT_IDLE) //если запустить без консоли, то здесь цикл со 100% загрузкой процессора
	}

	LogAdd(MESS_INFO, "Завершили работу")
}
