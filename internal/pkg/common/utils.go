package common

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/Shopify/gomail"
	log "github.com/sirupsen/logrus"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	//файл для хранения лога
	logFile            *os.File
	ipCoordinatesCache = map[string]struct {
		created time.Time
		values  [2]float64
	}{}
	mutex = sync.Mutex{}
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableQuote: true,
	})
	log.SetLevel(log.InfoLevel)
	logFile, _ = os.OpenFile(LogName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
	log.SetOutput(io.MultiWriter(logFile, os.Stdout))
}

func ReOpenLogFile() {
	logFile, _ = os.OpenFile(LogName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	log.SetOutput(io.MultiWriter(logFile, os.Stdout))
	log.Infof("truncate log file")
}

func CloseLogFile() {
	log.SetOutput(os.Stdout)

	if logFile != nil {
		if err := logFile.Close(); err != nil {
			log.Warnf("closing logs: %s", err.Error())
		}
	}

	logFile = nil
}

func RotateLogFiles() {
	fs, err := os.Stat(LogName)
	if err != nil {
		return
	}

	if fs.Size() > maxLogFileMb*1024*1024 {
		CloseLogFile()

		if err = os.Rename(LogName, LogName+".old"); err != nil {
			return
		}

		ReOpenLogFile()
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
			counterData.CounterDayWeekClients[time.Now().Weekday()] = counterData.CounterClients[counterData.currentPos.Hour()]

			counterData.CounterDayBytes[(time.Now().Day() - 1)] = 0
			counterData.CounterDayConnections[(time.Now().Day() - 1)] = 0
			counterData.CounterDayClients[time.Now().Day()-1] = counterData.CounterClients[counterData.currentPos.Hour()]

			counterData.CounterDayYearBytes[(time.Now().YearDay() - 1)] = 0
			counterData.CounterDayYearConnections[(time.Now().YearDay() - 1)] = 0
			counterData.CounterDayYearClients[time.Now().YearDay()-1] = counterData.CounterClients[counterData.currentPos.Hour()]

			if time.Now().Month() != counterData.currentPos.Month() {
				counterData.CounterMonthBytes[int(time.Now().Month()-1)] = 0
				counterData.CounterMonthConnections[int(time.Now().Month()-1)] = 0
				counterData.CounterMonthClients[time.Now().Month()-1] = counterData.CounterClients[counterData.currentPos.Hour()]
			}
		}

		SaveCounters()
		counterData.currentPos = now
	}
}

func SaveCounters() {
	if err := SaveFile(CountersFilename, &counterData); err != nil {
		log.Errorf("saving counters: %s", err.Error())
	}
}

func LoadCounters() {
	if err := LoadFile(CountersFilename, &counterData); err != nil {
		log.Errorf("loading counters: %s", err.Error())
	}

	counterData.currentPos = time.Now()
	counterData.CounterClients[counterData.currentPos.Hour()] = 0
}

func AddCounter(bytes uint64) {
	counterData.mutex.Lock()
	defer counterData.mutex.Unlock()

	counterData.CounterBytes[counterData.currentPos.Hour()] = counterData.CounterBytes[counterData.currentPos.Hour()] + bytes
	counterData.CounterConnections[counterData.currentPos.Hour()] = counterData.CounterConnections[counterData.currentPos.Hour()] + 1

	counterData.CounterDayWeekBytes[int(counterData.currentPos.Weekday())] = counterData.CounterDayWeekBytes[int(counterData.currentPos.Weekday())] + bytes
	counterData.CounterDayWeekConnections[int(counterData.currentPos.Weekday())] = counterData.CounterDayWeekConnections[int(counterData.currentPos.Weekday())] + 1

	counterData.CounterDayBytes[(counterData.currentPos.Day() - 1)] = counterData.CounterDayBytes[(counterData.currentPos.Day()-1)] + bytes
	counterData.CounterDayConnections[(counterData.currentPos.Day() - 1)] = counterData.CounterDayConnections[(counterData.currentPos.Day()-1)] + 1

	counterData.CounterDayYearBytes[(counterData.currentPos.YearDay() - 1)] = counterData.CounterDayYearBytes[(counterData.currentPos.YearDay()-1)] + bytes
	counterData.CounterDayYearConnections[(counterData.currentPos.YearDay() - 1)] = counterData.CounterDayYearConnections[(counterData.currentPos.YearDay()-1)] + 1

	counterData.CounterMonthBytes[int(counterData.currentPos.Month()-1)] = counterData.CounterMonthBytes[int(counterData.currentPos.Month()-1)] + bytes
	counterData.CounterMonthConnections[int(counterData.currentPos.Month()-1)] = counterData.CounterMonthConnections[int(counterData.currentPos.Month()-1)] + 1
}

func UpdateCounterClient(adding bool) {

	counterData.mutex.Lock()
	defer counterData.mutex.Unlock()

	if adding {
		counterData.CounterClients[counterData.currentPos.Hour()] = counterData.CounterClients[counterData.currentPos.Hour()] + 1
	} else {
		counterData.CounterClients[counterData.currentPos.Hour()] = counterData.CounterClients[counterData.currentPos.Hour()] - 1
		return
	}

	count := counterData.CounterClients[counterData.currentPos.Hour()]

	if counterData.CounterDayWeekClients[int(counterData.currentPos.Weekday())] < count {
		counterData.CounterDayWeekClients[int(counterData.currentPos.Weekday())] = count
	}

	if counterData.CounterDayClients[(counterData.currentPos.Day()-1)] < count {
		counterData.CounterDayClients[(counterData.currentPos.Day() - 1)] = count
	}

	if counterData.CounterDayYearClients[(counterData.currentPos.YearDay()-1)] < count {
		counterData.CounterDayYearClients[(counterData.currentPos.YearDay() - 1)] = count
	}

	if counterData.CounterMonthClients[int(counterData.currentPos.Month()-1)] < count {
		counterData.CounterMonthClients[int(counterData.currentPos.Month()-1)] = count
	}
}

func GetCoordinatesByYandex(addr string) [2]float64 {
	mutex.Lock()
	defer mutex.Unlock()
	if v, ok := ipCoordinatesCache[addr]; ok {
		if int(time.Now().Sub(v.created).Hours()) < int(time.Hour)*Options.IpCoordinateCacheTimeoutHours {
			return v.values
		}
	}

	resp, err := http.Post(UriYandexMap, "application/x-www-form-urlencoded", strings.NewReader(fmt.Sprintf("json=%s", url.QueryEscape(fmt.Sprintf(ReqYandexMap, Options.YandexApiKeyMap, addr)))))
	if err != nil {
		//todo надо мой айпи адрес как-то указать
		return [2]float64{0, 0}
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		//todo надо мой айпи адрес как-то указать
		return [2]float64{0, 0}
	}

	var respYandex YandexResp
	if err = json.Unmarshal(b, &respYandex); err != nil {
		//todo надо мой айпи адрес как-то указать
		return [2]float64{0, 0}
	}

	if len(respYandex.Error.Message) > 0 {
		log.Warnf("yandex geo: %s", respYandex.Error.Message)
		return [2]float64{0, 0}
	}

	item := ipCoordinatesCache[addr]
	item.created = time.Now()
	item.values = [2]float64{respYandex.Position.Latitude, respYandex.Position.Longitude}
	ipCoordinatesCache[addr] = item
	return item.values
}

func CleanOldCacheCoordinates() {
	mutex.Lock()
	defer mutex.Unlock()

	for k, v := range ipCoordinatesCache {
		if int(time.Now().Sub(v.created).Hours()) > int(time.Hour)*Options.IpCoordinateCacheTimeoutHours {
			delete(ipCoordinatesCache, k)
		}
	}
}

func GetSHA256(str string) string {

	s := sha256.Sum256([]byte(str))
	var r string

	for _, x := range s {
		r = r + fmt.Sprintf("%02x", x)
	}

	return r
}

func SendEmail(to, subject, body string) (bool, error) {
	p, _ := strconv.Atoi(Options.PortSMTP)
	d := gomail.NewDialer(Options.ServerSMTP, p, Options.LoginSMTP, Options.PassSMTP)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	m := gomail.NewMessage()
	m.SetHeader("From", Options.LoginSMTP)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html; charset=utf-8", body)
	if err := d.DialAndSend(m); err != nil {
		return false, err
	}

	return true, nil
}

func GetCounterHour() []string {
	return getCounter(counterData.CounterBytes[:], counterData.CounterConnections[:], counterData.CounterClients[:], 24, counterData.currentPos.Hour())
}

func GetCounterDayWeek() []string {
	return getCounter(counterData.CounterDayWeekBytes[:], counterData.CounterDayWeekConnections[:], counterData.CounterDayWeekClients[:], 7, int(counterData.currentPos.Weekday()))
}

func GetCounterDay() []string {
	return getCounter(counterData.CounterDayBytes[:], counterData.CounterDayConnections[:], counterData.CounterDayClients[:], 31, counterData.currentPos.Day()-1)
}

func GetCounterDayYear() []string {
	return getCounter(counterData.CounterDayYearBytes[:], counterData.CounterDayYearConnections[:], counterData.CounterDayYearClients[:], 365, counterData.currentPos.YearDay()-1)
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
		stringValues1 = stringValues1 + fmt.Sprint(values1[i]/1024/1024) //in Mb
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
		log.Errorf("не получилось получить список сетевых интерфейсов: %s", err.Error())
		return ""
	}

	ip := net.IPv4zero.String()
	for _, item := range interfaces {
		if (item.Flags&net.FlagLoopback == 0) && (item.Flags&net.FlagPointToPoint == 0) && (item.Flags&net.FlagUp == 1) {
			z, err := item.Addrs()
			if err != nil {
				log.Errorf("не получилось узнать IP для сетевого интерфейса: %s", err.Error())
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

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		//todo надо мой айпи адрес как-то указать
		return ""
	}

	return string(b)
}

func SaveFile(name string, data interface{}) error {
	b, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	_ = os.Remove(fmt.Sprintf("%s.tmp", name))
	_ = os.Rename(name, fmt.Sprintf("%s.tmp", name))

	f, err := os.Create(fmt.Sprintf("%s", name))
	if err != nil {
		return fmt.Errorf("saving: %w", err)
	}

	defer func() {
		if err = f.Close(); err != nil {
			log.Errorf("closing: %s", err.Error())
		}
	}()

	n, err := f.Write(b)
	if n == len(b) && err == nil {
		return nil
	} else if err != nil {
		return fmt.Errorf("writing: %w", err)
	} else {
		return nil
	}
}

func LoadFile(name string, data interface{}) error {
	f, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("opening: %w", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Errorf("closing: %s", err.Error())
		}
	}()

	b, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	if err = json.Unmarshal(b, data); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	return nil
}
