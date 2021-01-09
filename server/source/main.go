package main

import (
	"./common"
	"./component/profile"
	"./service"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	for _, x := range os.Args {
		if strings.Contains(x, "node") {
			common.Options.Mode = common.ModeNode
		} else if strings.Contains(x, "master") {
			common.Options.Mode = common.ModeMaster
		}
	}

	common.LogAdd(common.MessInfo, "Запускается сервер reVisit версии "+common.ReVisitVersion)

	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	common.LoadOptions()

	if common.Options.Mode != common.ModeNode {
		service.LoadVNCList()
		common.LoadCounters()
		profile.LoadProfiles()

		go service.HelperThread() //используем для периодических действий(сохранения и т.п.)
		go service.HttpServer()   //обработка веб запросов
		go service.MainServer()   //обработка основных команд от клиентов и агентов
	}

	service.UpdateMyIP()

	if common.Options.Mode != common.ModeMaster {
		go service.DataServer() //обработка потоков данных от клиентов
	}

	if common.Options.Mode == common.ModeMaster {
		go service.MasterServer() //общаемся с агентами
	}

	if common.Options.Mode == common.ModeNode {
		go service.NodeClient() //клиент подключающийся к мастеру
	}

	killSignal := <-interrupt
	switch killSignal {
	case os.Interrupt:
		common.LogAdd(common.MessInfo, "got SIGINT...")
	case syscall.SIGTERM:
		common.LogAdd(common.MessInfo, "got SIGTERM...")
	}

	common.LogAdd(common.MessInfo, "Завершили работу")
}
