package common

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	LOG_NAME = "log.txt"
	LOG_SIZE = 20 * 1024 * 1024 //20MB

	//виды сообщений логов
	MESS_ERROR  = 1
	MESS_INFO   = 2
	MESS_DETAIL = 3
	MESS_FULL   = 4
)

var (
	//максимальный уровень логов
	typeLog = MESS_FULL

	//файл для хранения лога
	logFile *os.File

	//текстовая расшифровка сообщений для логов
	messLogText = []string{
		"BLANK",
		"ERROR",
		"INFO",
		"DETAIL",
		"FULL"}
)

func LogAdd(TMessage int, Messages string) {
	if Options.FDebug && typeLog >= TMessage {

		if logFile == nil {
			var err error
			if Options.Mode == NODE {
				logFile, err = os.Create("node_" + LOG_NAME)
			} else {
				logFile, err = os.Create(LOG_NAME)
			}
			if err != nil {
				fmt.Println(fmt.Sprint(time.Now().Format("02 Jan 2006 15:04:05.000000")) + "\t" + messLogText[MESS_ERROR] + ":\tcouldn't create log file: " + fmt.Sprint(err))
				return
			}
		}

		//todo наверное стоит убрать, но пока мешает пинг в логах
		if strings.Contains(Messages, "buff (31): {\"TMessage\":18,\"Messages\":null}") || strings.Contains(Messages, "{18 []}") {
			return
		}

		logFile.Write([]byte(fmt.Sprint(time.Now().Format("02 Jan 2006 15:04:05.000000")) + "\t" + messLogText[TMessage] + ":\t" + Messages + "\n"))
		fmt.Println(fmt.Sprint(time.Now().Format("02 Jan 2006 15:04:05.000000")) + "\t" + messLogText[TMessage] + ":\t" + Messages)

		fs, err := logFile.Stat()
		if err != nil {
			return
		}
		if fs.Size() > LOG_SIZE {
			logFile.Close()
			if Options.Mode == NODE {
				os.Rename("node_"+LOG_NAME, "node_"+LOG_NAME+".back")
			} else {
				os.Rename(LOG_NAME, LOG_NAME+".back")
			}
			logFile = nil
		}
	}

}

func ClearLog(){
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}