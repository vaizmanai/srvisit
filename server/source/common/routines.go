package common

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
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
	if Options.DebugFlag && Options.LogLevel >= TMessage {

		if logFile == nil {
			var err error
			if Options.Mode == ModeNode {
				logFile, err = os.Create("node_" + LogFilename)
			} else {
				logFile, err = os.Create(LogFilename)
			}
			if err != nil {
				fmt.Println(fmt.Sprint(time.Now().Format("02 Jan 2006 15:04:05.000000")) + "\t" + messLogText[MessError] + ":\tcouldn't create log file: " + fmt.Sprint(err))
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
		if fs.Size() > LogSize {
			logFile.Close()
			if Options.Mode == ModeNode {
				os.Rename("node_"+LogFilename, "node_"+LogFilename+".back")
			} else {
				os.Rename(LogFilename, LogFilename+".back")
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
	b, err := json.MarshalIndent(&counterData, "", "  ")
	if err == nil {
		f, err := os.Create(CountersFilename)
		if err == nil {
			n, err := f.Write(b)
			if n != len(b) || err != nil {
				LogAdd(MessError, "Не удалось сохранить счетчики: "+fmt.Sprint(err))
			}
			f.Close()
		} else {
			LogAdd(MessError, "Не удалось сохранить счетчики: "+fmt.Sprint(err))
		}
	} else {
		LogAdd(MessError, "Не удалось сохранить счетчики: "+fmt.Sprint(err))
	}
}

func LoadCounters() {
	counterData.currentPos = time.Now()

	f, err := os.Open(CountersFilename)
	defer f.Close()
	if err == nil {
		b, err := ioutil.ReadAll(f)
		if err == nil {
			err = json.Unmarshal(b, &counterData)
			if err != nil {
				LogAdd(MessError, "Не получилось загрузить счетчики: "+fmt.Sprint(err))
			}
		} else {
			LogAdd(MessError, "Не получилось загрузить счетчики: "+fmt.Sprint(err))
		}
	} else {
		LogAdd(MessError, "Не получилось загрузить счетчики: "+fmt.Sprint(err))
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

func UpdateCounterClient(adding bool) {

	counterData.mutex.Lock()
	defer counterData.mutex.Unlock()

	if adding {
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
	resp, err := http.Post(UriYandexMap, "application/x-www-form-urlencoded", strings.NewReader("json="+url.QueryEscape(fmt.Sprintf(ReqYandexMap, Options.YandexApiKeyMap, addr))))
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

func GetCounterHour() []string {
	return getCounter(counterData.CounterBytes[:], counterData.CounterConnections[:], counterData.CounterClients[:], 24, int(counterData.currentPos.Hour()))
}

func GetCounterDayWeek() []string {
	return getCounter(counterData.CounterDayWeekBytes[:], counterData.CounterDayWeekConnections[:], counterData.CounterDayWeekClients[:], 7, int(counterData.currentPos.Weekday()))
}

func GetCounterDay() []string {
	return getCounter(counterData.CounterDayBytes[:], counterData.CounterDayConnections[:], counterData.CounterDayClients[:], 31, int(counterData.currentPos.Day()-1))
}

func GetCounterDayYear() []string {
	return getCounter(counterData.CounterDayYearBytes[:], counterData.CounterDayYearConnections[:], counterData.CounterDayYearClients[:], 365, int(counterData.currentPos.YearDay()-1))
}

func GetCounterMonth() []string {
	return getCounter(counterData.CounterMonthBytes[:], counterData.CounterMonthConnections[:], counterData.CounterMonthClients[:], 12, int(counterData.currentPos.Month()-1))
}

func getCounter(bytes []uint64, connections []uint64, clients []uint64, maxIndex int, curIndex int) []string {
	h := curIndex + 1

	values1 := append(bytes[h:], bytes[:h]...)
	values2 := append(connections[h:], connections[:h]...)
	values3 := append(clients[h:], clients[:h]...)

	for i := 0; i < maxIndex; i++ {
		values1[i] = values1[i] / 2
		values2[i] = values2[i] / 2
	}

	headers := make([]int, 0)
	for i := h; i < maxIndex; i++ {
		headers = append(headers, i)
	}
	for i := 0; i < h; i++ {
		headers = append(headers, i)
	}

	stringHeaders := "["
	for i := 0; i < maxIndex; i++ {
		stringHeaders = stringHeaders + "'" + fmt.Sprint(headers[i]+1) + "'"
		if i != maxIndex-1 {
			stringHeaders = stringHeaders + ", "
		}
	}
	stringHeaders = stringHeaders + "]"

	stringValues1 := "["
	for i := 0; i < maxIndex; i++ {
		stringValues1 = stringValues1 + fmt.Sprint(values1[i]/1024) //in Kb
		if i != maxIndex-1 {
			stringValues1 = stringValues1 + ", "
		}
	}
	stringValues1 = stringValues1 + "]"

	stringValues2 := "["
	for i := 0; i < maxIndex; i++ {
		stringValues2 = stringValues2 + fmt.Sprint(values2[i])
		if i != maxIndex-1 {
			stringValues2 = stringValues2 + ", "
		}
	}
	stringValues2 = stringValues2 + "]"

	stringValues3 := "["
	for i := 0; i < maxIndex; i++ {
		stringValues3 = stringValues3 + fmt.Sprint(values3[i])
		if i != maxIndex-1 {
			stringValues3 = stringValues3 + ", "
		}
	}
	stringValues3 = stringValues3 + "]"

	answer := make([]string, 0)
	answer = append(answer, stringHeaders)
	answer = append(answer, stringValues1)
	answer = append(answer, stringValues2)
	answer = append(answer, stringValues3)

	return answer
}

func GetMyIp() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		LogAdd(MessError, "Не получилось получить список сетевых интерфейсов: "+fmt.Sprint(err))
		return ""
	}

	ip := net.IPv4zero.String()
	for _, item := range interfaces {
		if (item.Flags&net.FlagLoopback == 0) && (item.Flags&net.FlagPointToPoint == 0) && (item.Flags&net.FlagUp == 1) {
			z, err := item.Addrs()
			if err != nil {
				LogAdd(MessError, "Не получилось узнать IP для сетевого интерфейса: "+fmt.Sprint(err))
				return ""
			}

			for _, j := range z {
				x, _, _ := net.ParseCIDR(j.String())

				if x.IsGlobalUnicast() && x.To4() != nil {
					ip = x.To4().String()
					return ip
				}
			}
		}
	}

	return ip
}

func GetMyIpByExternalApi() string {
	resp, err := http.Get(UriServiceIpIFY)
	if err != nil {
		//todo надо мой айпи адрес как-то указать
		return ""
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		//todo надо мой айпи адрес как-то указать
		return ""
	}

	return string(b)
}
