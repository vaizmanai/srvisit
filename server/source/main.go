package main

import (
	. "./common"
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
	clients = make(map[string][]*Client, 0)

	LoadOptions()

	if Options.Mode != NODE {
		loadVNCList()
		loadCounters()
		loadProfiles()

		go helperThread() //используем для периодических действий(сохранения и т.п.)
		go HttpServer()   //обработка веб запросов
		go mainServer()   //обработка основных команд от клиентов и агентов
	}

	myIp = getMyIpByExternalApi()
	if Options.MyCoordinates == [2]float64{0, 0} { //options.MyCoordinates[0] == 0 && options.MyCoordinates[1] == 0 {
		coordinates = getCoordinatesByYandex(myIp)
	} else {
		coordinates = Options.MyCoordinates
	}

	if Options.Mode != MASTER {
		go dataServer() //обработка потоков данных от клиентов
	}

	if Options.Mode == MASTER {
		go masterServer() //общаемся с агентами
	}

	if Options.Mode == NODE {
		go nodeClient() //клинет подключающийся к мастеру
	}

	var r string
	for r != "quit" {
		fmt.Scanln(&r)
		time.Sleep(time.Millisecond * WAIT_IDLE) //если запустить без консоли, то здесь цикл со 100% загрузкой процессора
	}

	LogAdd(MESS_INFO, "Завершили работу")
}
