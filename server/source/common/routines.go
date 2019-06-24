package common

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"time"
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

func ClearLog() {
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

func GetPid(serial string) string {

	var a uint64 = 1
	for _, f := range serial {
		a = a * uint64(f)
	}

	//todo факир был пьян, но пока и так сойдет
	b := a % 999
	if b == 0 {
		b = 1
	}
	for b < 100 {
		b = b * 10
	}
	c := (a / 999) % 999
	if c == 0 {
		c = 1
	}
	for c < 100 {
		c = c * 10
	}
	d := ((a / 999) / 999) % 999
	if d == 0 {
		d = 1
	}
	for d < 100 {
		d = d * 10
	}
	e := (((a / 999) / 999) / 999) % 999
	if e == 0 {
		e = 1
	}
	for e < 100 {
		e = e * 10
	}

	return fmt.Sprintf("%d:%d:%d:%d", b, c, d, e)
}

func CleanPid(pid string) string {
	//todo может потом стоит сюда добавить удаление и других символов
	return strings.Replace(pid, ":", "", -1)
}

func RandomString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(RandInt(65, 91))
	}
	return string(bytes)
}

func RandInt(min int, max int) int {
	return min + rand.Intn(max-min)
}

func SwiftCounter() {
	counterData.mutex.Lock()
	defer counterData.mutex.Unlock()

	if time.Now().Hour() != counterData.currentPos.Hour() {
		now := time.Now()
		counterData.CounterBytes[time.Now().Hour()] = 0
		counterData.CounterConnections[time.Now().Hour()] = 0
		counterData.CounterClients[time.Now().Hour()] = counterData.CounterClients[counterData.currentPos.Hour()]

		if time.Now().Day() != counterData.currentPos.Day() {
			counterData.CounterDayWeekBytes[int(time.Now().Weekday())] = 0
			counterData.CounterDayWeekConnections[int(time.Now().Weekday())] = 0
			counterData.CounterDayWeekClients[time.Now().Weekday()] = counterData.CounterClients[int(counterData.currentPos.Hour())]

			counterData.CounterDayBytes[int(time.Now().Day()-1)] = 0
			counterData.CounterDayConnections[int(time.Now().Day()-1)] = 0
			counterData.CounterDayClients[time.Now().Day()-1] = counterData.CounterClients[int(counterData.currentPos.Hour())]

			counterData.CounterDayYearBytes[int(time.Now().YearDay()-1)] = 0
			counterData.CounterDayYearConnections[int(time.Now().YearDay()-1)] = 0
			counterData.CounterDayYearClients[time.Now().YearDay()-1] = counterData.CounterClients[int(counterData.currentPos.Hour())]

			if time.Now().Month() != counterData.currentPos.Month() {
				counterData.CounterMonthBytes[int(time.Now().Month()-1)] = 0
				counterData.CounterMonthConnections[int(time.Now().Month()-1)] = 0
				counterData.CounterMonthClients[time.Now().Month()-1] = counterData.CounterClients[int(counterData.currentPos.Hour())]
			}
		}

		SaveCounters()
		counterData.currentPos = now
	}
}

func SaveCounters() {
	b, err := json.Marshal(&counterData)
	if err == nil {
		f, err := os.Create(FILE_COUNTERS)
		if err == nil {
			n, err := f.Write(b)
			if n != len(b) || err != nil {
				LogAdd(MESS_ERROR, "Не удалось сохранить счетчики: "+fmt.Sprint(err))
			}
			f.Close()
		} else {
			LogAdd(MESS_ERROR, "Не удалось сохранить счетчики: "+fmt.Sprint(err))
		}
	} else {
		LogAdd(MESS_ERROR, "Не удалось сохранить счетчики: "+fmt.Sprint(err))
	}
}

func LoadCounters() {
	counterData.currentPos = time.Now()

	f, err := os.Open(FILE_COUNTERS)
	defer f.Close()
	if err == nil {
		b, err := ioutil.ReadAll(f)
		if err == nil {
			err = json.Unmarshal(b, &counterData)
			if err != nil {
				LogAdd(MESS_ERROR, "Не получилось загрузить счетчики: "+fmt.Sprint(err))
			}
		} else {
			LogAdd(MESS_ERROR, "Не получилось загрузить счетчики: "+fmt.Sprint(err))
		}
	} else {
		LogAdd(MESS_ERROR, "Не получилось загрузить счетчики: "+fmt.Sprint(err))
	}

	counterData.CounterClients[int(counterData.currentPos.Hour())] = 0
}

func AddCounter(bytes uint64) {
	counterData.mutex.Lock()
	defer counterData.mutex.Unlock()

	counterData.CounterBytes[int(counterData.currentPos.Hour())] = counterData.CounterBytes[int(counterData.currentPos.Hour())] + bytes
	counterData.CounterConnections[int(counterData.currentPos.Hour())] = counterData.CounterConnections[int(counterData.currentPos.Hour())] + 1

	counterData.CounterDayWeekBytes[int(counterData.currentPos.Weekday())] = counterData.CounterDayWeekBytes[int(counterData.currentPos.Weekday())] + bytes
	counterData.CounterDayWeekConnections[int(counterData.currentPos.Weekday())] = counterData.CounterDayWeekConnections[int(counterData.currentPos.Weekday())] + 1

	counterData.CounterDayBytes[int(counterData.currentPos.Day()-1)] = counterData.CounterDayBytes[int(counterData.currentPos.Day()-1)] + bytes
	counterData.CounterDayConnections[int(counterData.currentPos.Day()-1)] = counterData.CounterDayConnections[int(counterData.currentPos.Day()-1)] + 1

	counterData.CounterDayYearBytes[int(counterData.currentPos.YearDay()-1)] = counterData.CounterDayYearBytes[int(counterData.currentPos.YearDay()-1)] + bytes
	counterData.CounterDayYearConnections[int(counterData.currentPos.YearDay()-1)] = counterData.CounterDayYearConnections[int(counterData.currentPos.YearDay()-1)] + 1

	counterData.CounterMonthBytes[int(counterData.currentPos.Month()-1)] = counterData.CounterMonthBytes[int(counterData.currentPos.Month()-1)] + bytes
	counterData.CounterMonthConnections[int(counterData.currentPos.Month()-1)] = counterData.CounterMonthConnections[int(counterData.currentPos.Month()-1)] + 1
}

func UpdateCounterClient(add bool) {

	counterData.mutex.Lock()
	defer counterData.mutex.Unlock()

	if add {
		counterData.CounterClients[int(counterData.currentPos.Hour())] = counterData.CounterClients[int(counterData.currentPos.Hour())] + 1
	} else {
		counterData.CounterClients[int(counterData.currentPos.Hour())] = counterData.CounterClients[int(counterData.currentPos.Hour())] - 1
		return
	}

	count := counterData.CounterClients[int(counterData.currentPos.Hour())]

	if counterData.CounterDayWeekClients[int(counterData.currentPos.Weekday())] < count {
		counterData.CounterDayWeekClients[int(counterData.currentPos.Weekday())] = count
	}

	if counterData.CounterDayClients[int(counterData.currentPos.Day()-1)] < count {
		counterData.CounterDayClients[int(counterData.currentPos.Day()-1)] = count
	}

	if counterData.CounterDayYearClients[int(counterData.currentPos.YearDay()-1)] < count {
		counterData.CounterDayYearClients[int(counterData.currentPos.YearDay()-1)] = count
	}

	if counterData.CounterMonthClients[int(counterData.currentPos.Month()-1)] < count {
		counterData.CounterMonthClients[int(counterData.currentPos.Month()-1)] = count
	}
}

func GetCoordinatesByYandex(addr string) [2]float64 {
	resp, err := http.Post(URI_YANDEX_MAP, "application/x-www-form-urlencoded", strings.NewReader("json="+url.QueryEscape(fmt.Sprintf(REQ_YANDEX_MAP, Options.YandexApiKeyMap, addr))))
	if err != nil {
		//todo надо мой айпи адрес как-то указать
		return [2]float64{0, 0}
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		//todo надо мой айпи адрес как-то указать
		return [2]float64{0, 0}
	}

	var respYandex YandexResp
	err = json.Unmarshal(b, &respYandex)
	if err != nil {
		//todo надо мой айпи адрес как-то указать
		return [2]float64{0, 0}
	}

	return [2]float64{respYandex.Position.Latitude, respYandex.Position.Longitude}
}

func GetSHA256(str string) string {

	s := sha256.Sum256([]byte(str))
	var r string

	for _, x := range s {
		r = r + fmt.Sprintf("%02x", x)
	}

	return r
}

func SendEmail(to string, body string) (bool, error) {
	emailConn, err := tls.Dial("tcp", Options.ServerSMTP+":"+Options.PortSMTP, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return false, err
	}

	defer emailConn.Close()

	client, err := smtp.NewClient(emailConn, Options.ServerSMTP)
	if err != nil {
		return false, err
	}

	err = client.Auth(smtp.PlainAuth("", Options.LoginSMTP, Options.PassSMTP, Options.ServerSMTP))
	if err != nil {
		return false, err
	}

	err = client.Mail(Options.LoginSMTP)
	if err != nil {
		return false, err
	}

	err = client.Rcpt(to)
	if err != nil {
		return false, err
	}

	wc, err := client.Data()
	if err != nil {
		return false, err
	}

	defer wc.Close()
	wc.Write([]byte(body))

	err = client.Quit()
	return true, err
}
