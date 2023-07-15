package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
	"github.com/vaizmanai/srvisit/internal/pkg/processor"
	"github.com/vaizmanai/srvisit/internal/pkg/profile"
	"math/rand"
	"os"
	"os/signal"
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

	log.Infof("запускается сервер reVisit версии %s", common.WhitelabelVersion)

	rand.Seed(time.Now().UTC().UnixNano())

	common.LoadOptions()

	if common.Options.Mode != common.ModeNode {
		processor.LoadVNCList()
		common.LoadCounters()
		profile.LoadProfiles()

		go processor.HelperThread() //используем для периодических действий(сохранения и т.п.)
		go processor.MainServer()   //обработка основных команд от клиентов и агентов
		go processor.HttpServer()   //обработка веб запросов
	}

	processor.UpdateMyIP()

	if common.Options.Mode != common.ModeMaster {
		go processor.DataServer() //обработка потоков данных от клиентов
	}

	if common.Options.Mode == common.ModeMaster {
		go processor.MasterServer() //общаемся с агентами
	}

	if common.Options.Mode == common.ModeNode {
		go processor.NodeClient() //клиент подключающийся к мастеру
	}

	killSignal := <-interrupt
	switch killSignal {
	case os.Interrupt:
		log.Infof("got SIGINT...")
	case syscall.SIGTERM:
		log.Infof("got SIGTERM...")
	}

	log.Infof("завершили работу")
}
