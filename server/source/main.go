package main

import (
	. "./common"
	"./component/profile"
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
			Options.Mode = ModeNode
		} else if strings.Contains(x, "master") {
			Options.Mode = ModeMaster
		}
	}

	LogAdd(MessInfo, "Запускается сервер reVisit версии "+ReVisitVersion)

	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	LoadOptions()

	if Options.Mode != ModeNode {
		service.LoadVNCList()
		LoadCounters()
		profile.LoadProfiles()

		go service.HelperThread() //используем для периодических действий(сохранения и т.п.)
		go service.HttpServer()   //обработка веб запросов
		go service.MainServer()   //обработка основных команд от клиентов и агентов
	}

	service.UpdateMyIP()

	if Options.Mode != ModeMaster {
		go service.DataServer() //обработка потоков данных от клиентов
	}

	if Options.Mode == ModeMaster {
		go service.MasterServer() //общаемся с агентами
	}

	if Options.Mode == ModeNode {
		go service.NodeClient() //клинет подключающийся к мастеру
	}

	var r string
	for r != "quit" {
		fmt.Scanln(&r)
		time.Sleep(time.Millisecond * WaitIdle) //если запустить без консоли, то здесь цикл со 100% загрузкой процессора
	}

	LogAdd(MessInfo, "Завершили работу")
}
