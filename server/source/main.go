package main

import (
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
			options.mode = NODE
		} else if strings.Contains(x, "master") {
			options.mode = MASTER
		}
	}

	logAdd(MESS_INFO, "Запускается сервер reVisit версии "+REVISIT_VERSION)

	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())
	clients = make(map[string][]*Client, 0)

	loadOptions()

	if options.mode != NODE {
		loadVNCList()
		loadCounters()
		loadProfiles()

		go helperThread() //используем для периодических действий(сохранения и т.п.)
		go httpServer()   //обработка веб запросов
		go mainServer()   //обработка основных команд от клиентов и агентов
	}

	myIp = getMyIpByExternalApi()
	if options.MyCoordinates == [2]float64{0, 0} { //options.MyCoordinates[0] == 0 && options.MyCoordinates[1] == 0 {
		coordinates = getCoordinatesByYandex(myIp)
	} else {
		coordinates = options.MyCoordinates
	}

	if options.mode != MASTER {
		go dataServer() //обработка потоков данных от клиентов
	}

	if options.mode == MASTER {
		go masterServer() //общаемся с агентами
	}

	if options.mode == NODE {
		go nodeClient() //клинет подключающийся к мастеру
	}

	var r string
	for r != "quit" {
		fmt.Scanln(&r)
		time.Sleep(time.Millisecond * WAIT_IDLE) //если запустить без консоли, то здесь цикл со 100% загрузкой процессора
	}

	logAdd(MESS_INFO, "Завершили работу")
}
