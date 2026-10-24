package common

import (
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/Shopify/gomail"
	log "github.com/sirupsen/logrus"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	//файл для хранения лога
	logFile            *os.File
	logMutex           sync.Mutex
	externalHTTPClient = &http.Client{Timeout: 10 * time.Second}
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
	logFile, _ = os.OpenFile(LogName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	log.SetOutput(logWriter{})
}

type logWriter struct{}

func (logWriter) Write(p []byte) (int, error) {
	logMutex.Lock()
	defer logMutex.Unlock()
	if logFile != nil {
		// A file error must not suppress output to the console.
		_, _ = logFile.Write(p)
	}
	return os.Stdout.Write(p)
}

func openLogFileLocked(flags int) error {
	if logFile != nil {
		_ = logFile.Close()
	}
	var err error
	logFile, err = os.OpenFile(LogName, os.O_CREATE|os.O_WRONLY|flags, 0600)
	return err
}

func ReOpenLogFile() {
	logMutex.Lock()
	err := openLogFileLocked(os.O_TRUNC)
	logMutex.Unlock()
	if err != nil {
		log.Errorf("opening logs: %s", err)
	}
}

func CloseLogFile() {
	logMutex.Lock()
	defer logMutex.Unlock()
	if logFile != nil {
		_ = logFile.Close()
	}
	logFile = nil
}

func RotateLogFiles() {
	logMutex.Lock()
	fs, err := os.Stat(LogName)
	if err != nil || fs.Size() <= maxLogFileMb*1024*1024 {
		logMutex.Unlock()
		return
	}
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
	err = os.Rename(LogName, LogName+".old")
	openErr := openLogFileLocked(os.O_APPEND)
	logMutex.Unlock()
	if err != nil {
		log.Errorf("rotating logs: %s", err)
	}
	if openErr != nil {
		log.Errorf("opening logs: %s", openErr)
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
	result := make([]byte, l)
	entropy := make([]byte, l)
	for n := 0; n < l; {
		if _, err := cryptorand.Read(entropy); err != nil {
			panic(err)
		}
		for _, b := range entropy {
			// Reject the incomplete alphabet at the end of the byte range.
			if b >= 26*9 {
				continue
			}
			result[n] = 'A' + b%26
			n++
			if n == l {
				break
			}
		}
	}
	return string(result)
}

func RandInt(min int, max int) int {
	return min + rand.IntN(max-min)
}

func SwiftCounter() {
	counterMutex.Lock()
	defer counterMutex.Unlock()

	now := time.Now()
	if !now.Truncate(time.Hour).Equal(counterData.currentPos.Truncate(time.Hour)) {
		counterData.CounterBytes[time.Now().Hour()] = 0
		counterData.CounterConnections[time.Now().Hour()] = 0
		counterData.CounterClients[time.Now().Hour()] = counterData.CounterClients[counterData.currentPos.Hour()]

		if now.Year() != counterData.currentPos.Year() || now.YearDay() != counterData.currentPos.YearDay() {
			counterData.CounterDayWeekBytes[int(time.Now().Weekday())] = 0
			counterData.CounterDayWeekConnections[int(time.Now().Weekday())] = 0
			counterData.CounterDayWeekClients[time.Now().Weekday()] = counterData.CounterClients[counterData.currentPos.Hour()]

			counterData.CounterDayBytes[(time.Now().Day() - 1)] = 0
			counterData.CounterDayConnections[(time.Now().Day() - 1)] = 0
			counterData.CounterDayClients[time.Now().Day()-1] = counterData.CounterClients[counterData.currentPos.Hour()]

			counterData.CounterDayYearBytes[(time.Now().YearDay() - 1)] = 0
			counterData.CounterDayYearConnections[(time.Now().YearDay() - 1)] = 0
			counterData.CounterDayYearClients[time.Now().YearDay()-1] = counterData.CounterClients[counterData.currentPos.Hour()]

			if now.Year() != counterData.currentPos.Year() || now.Month() != counterData.currentPos.Month() {
				counterData.CounterMonthBytes[int(time.Now().Month()-1)] = 0
				counterData.CounterMonthConnections[int(time.Now().Month()-1)] = 0
				counterData.CounterMonthClients[time.Now().Month()-1] = counterData.CounterClients[counterData.currentPos.Hour()]
			}
		}

		counterData.currentPos = now
		saveCountersLocked()
	}
}

func SaveCounters() {
	counterMutex.Lock()
	defer counterMutex.Unlock()
	saveCountersLocked()
}

func saveCountersLocked() {
	if err := SaveFile(CountersFilename, &counterData); err != nil {
		log.Errorf("saving counters: %s", err.Error())
	}
}

func LoadCounters() {
	counterMutex.Lock()
	defer counterMutex.Unlock()
	if err := LoadFile(CountersFilename, &counterData); err != nil {
		log.Errorf("loading counters: %s", err.Error())
	}

	counterData.currentPos = time.Now()
	counterData.CounterClients[counterData.currentPos.Hour()] = 0
}

func AddCounter(bytes uint64) {
	counterMutex.Lock()
	defer counterMutex.Unlock()

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

	counterMutex.Lock()
	defer counterMutex.Unlock()

	if adding {
		counterData.CounterClients[counterData.currentPos.Hour()] = counterData.CounterClients[counterData.currentPos.Hour()] + 1
	} else {
		if counterData.CounterClients[counterData.currentPos.Hour()] > 0 {
			counterData.CounterClients[counterData.currentPos.Hour()]--
		}
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
	config := GetOptions()
	if config.YandexApiKeyMap == "" || net.ParseIP(addr) == nil {
		return [2]float64{}
	}
	ttl := time.Duration(config.IpCoordinateCacheTimeoutHours) * time.Hour
	mutex.Lock()
	if v, ok := ipCoordinatesCache[addr]; ok {
		if time.Since(v.created) < ttl {
			mutex.Unlock()
			return v.values
		}
	}
	mutex.Unlock()

	resp, err := externalHTTPClient.Post(UriYandexMap, "application/x-www-form-urlencoded", strings.NewReader(fmt.Sprintf("json=%s", url.QueryEscape(fmt.Sprintf(ReqYandexMap, config.YandexApiKeyMap, addr)))))
	if err != nil {
		//todo надо мой айпи адрес как-то указать
		return [2]float64{0, 0}
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return [2]float64{}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
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

	mutex.Lock()
	defer mutex.Unlock()
	item := ipCoordinatesCache[addr]
	item.created = time.Now()
	item.values = [2]float64{respYandex.Position.Latitude, respYandex.Position.Longitude}
	ipCoordinatesCache[addr] = item
	return item.values
}

func CleanOldCacheCoordinates() {
	ttl := time.Duration(GetOptions().IpCoordinateCacheTimeoutHours) * time.Hour
	mutex.Lock()
	defer mutex.Unlock()

	for k, v := range ipCoordinatesCache {
		if time.Since(v.created) >= ttl {
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
	config := GetOptions()
	p, err := strconv.Atoi(config.PortSMTP)
	if err != nil || p < 1 || p > 65535 || config.ServerSMTP == "" {
		return false, fmt.Errorf("invalid SMTP server or port")
	}
	d := gomail.NewDialer(config.ServerSMTP, p, config.LoginSMTP, config.PassSMTP)
	d.StartTLSPolicy = gomail.MandatoryStartTLS

	m := gomail.NewMessage()
	m.SetHeader("From", config.LoginSMTP)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html; charset=utf-8", body)
	if err := d.DialAndSend(m); err != nil {
		return false, err
	}

	return true, nil
}

func GetCounterHour() []string {
	counterMutex.Lock()
	defer counterMutex.Unlock()
	return getCounter(counterData.CounterBytes[:], counterData.CounterConnections[:], counterData.CounterClients[:], 24, counterData.currentPos.Hour())
}

func GetCounterDayWeek() []string {
	counterMutex.Lock()
	defer counterMutex.Unlock()
	return getCounter(counterData.CounterDayWeekBytes[:], counterData.CounterDayWeekConnections[:], counterData.CounterDayWeekClients[:], 7, int(counterData.currentPos.Weekday()))
}

func GetCounterDay() []string {
	counterMutex.Lock()
	defer counterMutex.Unlock()
	return getCounter(counterData.CounterDayBytes[:], counterData.CounterDayConnections[:], counterData.CounterDayClients[:], 31, counterData.currentPos.Day()-1)
}

func GetCounterDayYear() []string {
	counterMutex.Lock()
	defer counterMutex.Unlock()
	days := time.Date(counterData.currentPos.Year(), time.December, 31, 0, 0, 0, 0, time.UTC).YearDay()
	return getCounter(counterData.CounterDayYearBytes[:], counterData.CounterDayYearConnections[:], counterData.CounterDayYearClients[:], days, counterData.currentPos.YearDay()-1)
}

func GetCounterMonth() []string {
	counterMutex.Lock()
	defer counterMutex.Unlock()
	return getCounter(counterData.CounterMonthBytes[:], counterData.CounterMonthConnections[:], counterData.CounterMonthClients[:], 12, int(counterData.currentPos.Month()-1))
}

func getCounter(bytes []uint64, connections []uint64, clients []uint64, maxIndex int, curIndex int) []string {
	h := curIndex + 1

	values1 := append(append(make([]uint64, 0, maxIndex), bytes[h:maxIndex]...), bytes[:h]...)
	values2 := append(append(make([]uint64, 0, maxIndex), connections[h:maxIndex]...), connections[:h]...)
	values3 := append(append(make([]uint64, 0, maxIndex), clients[h:maxIndex]...), clients[:h]...)

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
	resp, err := externalHTTPClient.Get(UriServiceIpIFY)
	if err != nil {
		//todo надо мой айпи адрес как-то указать
		return ""
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		//todo надо мой айпи адрес как-то указать
		return ""
	}

	ip := strings.TrimSpace(string(b))
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

func SaveFile(name string, data interface{}) error {
	b, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	f, err := os.CreateTemp(filepath.Dir(name), "."+filepath.Base(name)+".*.tmp")
	if err != nil {
		return fmt.Errorf("saving: %w", err)
	}

	defer os.Remove(f.Name())
	defer f.Close()

	n, err := f.Write(b)
	if err != nil {
		return fmt.Errorf("writing: %w", err)
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("syncing: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing: %w", err)
	}
	if err := os.Rename(f.Name(), name); err != nil {
		return fmt.Errorf("replacing: %w", err)
	}
	return nil
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
