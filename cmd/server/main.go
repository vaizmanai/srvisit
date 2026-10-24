package main

import (
	"errors"
	"flag"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
	"github.com/vaizmanai/srvisit/internal/pkg/processor"
	"github.com/vaizmanai/srvisit/internal/pkg/profile"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)
	defer common.CloseLogFile()

	if err := common.LoadOptions(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Fatalf("loading options: %s", err)
		}
		if err := common.SaveOptions(); err != nil {
			log.Fatalf("creating options: %s", err)
		}
		log.Infof("created %s; administrator and master credentials are stored there", common.FileOptions)
	}
	node := flag.Bool("node", false, "run as a data node")
	master := flag.Bool("master", false, "run as a master server")
	flag.Parse()
	if *node && *master {
		log.Fatal("choose either -node or -master")
	}
	config := common.GetOptions()
	if *node {
		config.Mode = common.ModeNode
	} else if *master {
		config.Mode = common.ModeMaster
	}
	if err := common.SetOptions(config); err != nil {
		log.Fatal(err)
	}
	log.SetLevel(config.LogLevel)

	log.Infof("запускается сервер %s версии %s", common.WhitelabelName, common.WhitelabelVersion)

	processor.UpdateMyIP()

	if config.Mode != common.ModeNode {
		processor.LoadVNCList()
		common.LoadCounters()
		profile.LoadProfiles()

		go processor.HelperThread() //используем для периодических действий(сохранения и т.п.)
		go processor.MainServer()   //обработка основных команд от клиентов и агентов
		go processor.HttpServer()   //обработка веб запросов
	}

	if config.Mode != common.ModeMaster {
		go processor.DataServer() //обработка потоков данных от клиентов
	}

	if config.Mode == common.ModeMaster {
		go processor.MasterServer() //общаемся с агентами
	}

	if config.Mode == common.ModeNode {
		go processor.NodeClient() //клиент подключающийся к мастеру
	}

	killSignal := <-interrupt
	switch killSignal {
	case os.Interrupt:
		log.Infof("got SIGINT...")
	case syscall.SIGTERM:
		log.Infof("got SIGTERM...")
	}

	if config.Mode != common.ModeNode {
		profile.SaveProfiles()
		common.SaveCounters()
	}
	log.Infof("завершили работу")
}
