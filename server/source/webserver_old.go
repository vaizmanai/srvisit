package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

//хэндлеры для профиля
func handleProfileWelcome(w http.ResponseWriter, r *http.Request) {

	file, _ := os.Open("resource/profile/welcome.html")
	body, err := ioutil.ReadAll(file)
	if err == nil {
		file.Close()

		body = pageReplace(body, "$menu", addMenuProfile())
		w.Write(body)
		return
	}

}

func handleProfileMy(w http.ResponseWriter, r *http.Request) {
	curProfile := checkProfileAuth(w, r)

	if curProfile == nil {
		return
	}

	file, _ := os.Open("resource/profile/my.html")
	body, err := ioutil.ReadAll(file)
	if err == nil {
		file.Close()

		body = pageReplace(body, "$menu", addMenuProfile())
		w.Write(body)
		return
	}
}

//хэндлеры для админки
func handleWelcome(w http.ResponseWriter, r *http.Request) {

	file, _ := os.Open("resource/admin/welcome.html")
	body, err := ioutil.ReadAll(file)
	if err == nil {
		file.Close()

		body = pageReplace(body, "$menu", addMenuAdmin())
		w.Write(body)
		return
	}

}

func handleResources(w http.ResponseWriter, r *http.Request) {

	if !checkAdminAuth(w, r) {
		return
	}

	connectionsString := ""

	var buf1 string
	if options.mode == MASTER {
		connectionsString = connectionsString + fmt.Sprint("\n\n<a href='#'>агенты</a><br><pre>\n")
		nodes.Range(func(key interface{}, value interface{}) bool {
			agent := value.(*Node)
			connectionsString = connectionsString + fmt.Sprintf("%-16s%-26s%s\n", agent.Id, agent.Ip, agent.Name)
			return true
		})
		connectionsString = connectionsString + fmt.Sprint("</pre>\n")
	}

	connectionsString = connectionsString + fmt.Sprint("\n\n<a href='#' onclick='show(clients);'>клиенты</a><br>")
	connectionsString = connectionsString + fmt.Sprint("<div id='clients' class='hidden'><pre>")

	for _, list := range clients {
		for _, client := range list {
			if client.Profile == nil {
				buf1 = "no auth"
			} else {
				buf1 = client.Profile.Email
			}

			connectionsString = connectionsString + fmt.Sprintln(client.Pid, client.Serial, client.Version, (*client.Conn).RemoteAddr(), buf1)

			client.profiles.Range(func(k interface{}, v interface{}) bool {

				var capt string
				c := getContactByPid(v.(*Profile).Contacts, cleanPid(client.Pid)) //todo потом убрать, лишние итерации не сильно нам интересны
				if c != nil {
					capt = fmt.Sprint("/ ", c.Caption)
				}

				connectionsString = connectionsString + fmt.Sprintln("\t ->", k.(string), capt)

				return true
			})
		}
	}

	connectionsString = connectionsString + fmt.Sprint("</pre></div>\n\n<a href='#' onclick='show(sessions);'>сессии</a><br>")
	connectionsString = connectionsString + fmt.Sprint("<div id='sessions' class='hidden'><pre>")

	channels.Range(func(key interface{}, value interface{}) bool {
		dConn := value.(*dConn)

		connectionsString = connectionsString + fmt.Sprint(key.(string), "\n")

		var p1, p2, ps string

		if dConn.pointer[0] != nil {
			p1 = (*dConn.pointer[0]).RemoteAddr().String() + "(d)"
		} else {
			h, _, err := net.SplitHostPort((*dConn.client.Conn).RemoteAddr().String())
			if err == nil {
				p1 = h + "(m)"
			} else {
				p1 = (*dConn.client.Conn).RemoteAddr().String() + "(m)"
			}
		}

		if dConn.pointer[1] != nil {
			p2 = (*dConn.pointer[1]).RemoteAddr().String() + "(d)"
		} else {
			h, _, err := net.SplitHostPort((*dConn.server.Conn).RemoteAddr().String())
			if err == nil {
				p2 = h + "(m)"
			} else {
				p2 = (*dConn.server.Conn).RemoteAddr().String() + "(m)"
			}
		}

		if len(dConn.address) > 0 {
			ps = dConn.address + "(a)"
		} else if dConn.node != nil {
			ps = dConn.node.Ip + "(a)"
		} else {
			ps = myIp + "(m)"
		}

		connectionsString = connectionsString + fmt.Sprintf("%-26s%-26s%-26s\n", p1+" <", "> "+ps+" <", "> "+p2)

		connectionsString = connectionsString + fmt.Sprintf("%-54s%-54s\n", dConn.server.Pid+" "+dConn.server.Version, dConn.client.Pid+" "+dConn.client.Version)

		if dConn.server.Profile != nil {
			p1 = dConn.server.Profile.Email
		} else {
			p1 = ""
		}

		if dConn.client.Profile != nil {
			p2 = dConn.client.Profile.Email
		} else {
			p2 = ""
		}

		connectionsString = connectionsString + fmt.Sprintf("%-54s%-54s\n\n", p1, p2)
		return true
	})

	connectionsString = connectionsString + fmt.Sprint("\ta - agent\n\tm - main\n\td - data\n\n")

	connectionsString = connectionsString + fmt.Sprint("</pre></div>\n\n<a href='#' onclick='show(profiles);'>профили</a><br>")
	connectionsString = connectionsString + fmt.Sprint("<div id='profiles' class='hidden'><pre>")

	profiles.Range(func(key interface{}, value interface{}) bool {

		connectionsString = connectionsString + fmt.Sprintln(key.(string)) //(*value.(*Profile)).Pass)

		value.(*Profile).clients.Range(func(key interface{}, value interface{}) bool {
			connectionsString = connectionsString + fmt.Sprintln("\t", "<- "+key.(string))
			return true
		})

		return true
	})
	connectionsString = connectionsString + "</pre></div>"

	file, _ := os.Open("resource/admin/resources.html")
	body, err := ioutil.ReadAll(file)
	if err == nil {
		file.Close()

		body = pageReplace(body, "$menu", addMenuAdmin())
		body = pageReplace(body, "$connections", connectionsString)
		w.Write(body)
		return
	}

}

func handleStatistics(w http.ResponseWriter, r *http.Request) {

	if !checkAdminAuth(w, r) {
		return
	}

	file, _ := os.Open("resource/admin/statistics.html")
	body, err := ioutil.ReadAll(file)
	if err == nil {
		file.Close()

		body = pageReplace(body, "$menu", addMenuAdmin())
		body = pageReplace(body, "$clients", addClientsStatisticsAdmin())
		body = pageReplace(body, "$connections", addConnectionsAdmin())
		body = pageReplace(body, "$agents", addAgentsAdmin())

		charts := getCounterHour()
		body = pageReplace(body, "$headers01", charts[0]) //по часам
		body = pageReplace(body, "$values01", charts[1])
		body = pageReplace(body, "$values02", charts[2])
		body = pageReplace(body, "$values21", charts[3])

		charts = getCounterDayWeek()
		body = pageReplace(body, "$headers02", charts[0]) //по дням недели
		body = pageReplace(body, "$values03", charts[1])
		body = pageReplace(body, "$values04", charts[2])
		body = pageReplace(body, "$values22", charts[3])

		charts = getCounterDay()
		body = pageReplace(body, "$headers03", charts[0]) //по дням месяца
		body = pageReplace(body, "$values05", charts[1])
		body = pageReplace(body, "$values06", charts[2])
		body = pageReplace(body, "$values23", charts[3])

		charts = getCounterDayYear()
		body = pageReplace(body, "$headers04", charts[0]) //по дням года
		body = pageReplace(body, "$values07", charts[1])
		body = pageReplace(body, "$values08", charts[2])
		body = pageReplace(body, "$values24", charts[3])

		charts = getCounterMonth()
		body = pageReplace(body, "$headers05", charts[0]) //по месяцам
		body = pageReplace(body, "$values09", charts[1])
		body = pageReplace(body, "$values10", charts[2])
		body = pageReplace(body, "$values25", charts[3])

		w.Write(body)
		return
	}

}

func handleOptions(w http.ResponseWriter, r *http.Request) {

	if !checkAdminAuth(w, r) {
		return
	}

	file, _ := os.Open("resource/admin/options.html")
	body, err := ioutil.ReadAll(file)
	if err == nil {
		file.Close()

		body = pageReplace(body, "$menu", addMenuAdmin())
		//body = pageReplace(body, "$logs", logsString)
		w.Write(body)
		return
	}

}

func handleLogs(w http.ResponseWriter, r *http.Request) {

	if !checkAdminAuth(w, r) {
		return
	}

	file, _ := os.Open("resource/admin/logs.html")
	body, err := ioutil.ReadAll(file)
	if err == nil {
		file.Close()

		body = pageReplace(body, "$menu", addMenuAdmin())
		//body = pageReplace(body, "$logs", logsString)
		w.Write(body)
		return
	}

}

//ресурсы и api
func handleResource(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, r.URL.Path[1:])
}

func handleAPI(w http.ResponseWriter, r *http.Request) {

	actMake := r.URL.Query().Get("make")

	for _, m := range processingWeb {
		if actMake == m.Make {
			if m.Processing != nil {
				m.Processing(w, r)
			} else {
				logAdd(MESS_INFO, "WEB Нет обработчика для сообщения")
				time.Sleep(time.Millisecond * WAIT_IDLE)
			}
			return
		}
	}

	time.Sleep(time.Millisecond * WAIT_IDLE)
	logAdd(MESS_ERROR, "WEB Неизвестное сообщение")
	http.Error(w, "bad request", http.StatusBadRequest)
}

//раскрытие api
func processApiDefaultVnc(w http.ResponseWriter, r *http.Request) {
	logAdd(MESS_INFO, "WEB Запрос vnc версии по-умолчанию")

	if len(arrayVnc) < defaultVnc {
		buff, err := json.Marshal(arrayVnc[defaultVnc])
		if err != nil {
			logAdd(MESS_ERROR, "WEB Не получилось отправить версию VNC")
			return
		}
		w.Write(buff)
		return
	}
	http.Error(w, "vnc is not prepared", http.StatusNotAcceptable)
}

func processApiListVnc(w http.ResponseWriter, r *http.Request) {
	logAdd(MESS_INFO, "WEB Запрос списка vnc")

	buff, err := json.Marshal(arrayVnc)
	if err != nil {
		logAdd(MESS_ERROR, "WEB Не получилось отправить список VNC")
		return
	}
	w.Write(buff)
}

func processApiGetLog(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	logAdd(MESS_INFO, "WEB Запрос log")
	file, _ := os.Open(LOG_NAME)
	log, err := ioutil.ReadAll(file)
	if err == nil {
		file.Close()
	}
	w.Write(log)
}

func processApiClearLog(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	logAdd(MESS_INFO, "WEB Запрос очистки log")
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	http.Redirect(w, r, "/admin/logs", http.StatusTemporaryRedirect)
}

func processApiProfileSave(w http.ResponseWriter, r *http.Request) {
	curProfile := checkProfileAuth(w, r)
	if curProfile == nil {
		return
	}

	logAdd(MESS_INFO, "WEB Запрос сохранения профиля "+curProfile.Email)

	pass1 := string(r.FormValue("abc"))
	pass2 := string(r.FormValue("def"))

	capt := string(r.FormValue("capt"))
	tel := string(r.FormValue("tel"))
	logo := string(r.FormValue("logo"))

	if (pass1 != "*****") && (len(pass1) > 3) && (pass1 == pass2) {
		curProfile.Pass = pass1
	}
	curProfile.Capt = capt
	curProfile.Tel = tel
	curProfile.Logo = logo

	handleProfileMy(w, r)
}

func processApiProfileGet(w http.ResponseWriter, r *http.Request) {
	curProfile := checkProfileAuth(w, r)
	if curProfile == nil {
		return
	}

	logAdd(MESS_INFO, "WEB Запрос информации профиля "+curProfile.Email)

	newProfile := *curProfile
	newProfile.Pass = "*****"
	b, err := json.Marshal(&newProfile)
	if err == nil {
		w.Write(b)
		return
	}

	http.Error(w, "", http.StatusBadRequest)
}

func processApiSaveOptions(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	logAdd(MESS_INFO, "WEB Запрос сохранения опций")

	saveOptions()

	handleOptions(w, r)
}

func processApiReload(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	logAdd(MESS_INFO, "WEB Запрос на перезапуск сервера")

	//todo перезапуск
	w.WriteHeader(http.StatusOK)
}

func processApiReopen(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	logAdd(MESS_INFO, "WEB Запрос на чтение списка VNC")

	loadVNCList()
	w.WriteHeader(http.StatusOK)
}

func processApiVersion(w http.ResponseWriter, r *http.Request) {
	logAdd(MESS_INFO, "WEB Запрос актуальной версии")

	var resp [2]string
	resp[0] = options.Version
	fs, err := os.Stat("resource/reVisit.exe")
	if err != nil {
		logAdd(MESS_ERROR, "WEB Отсутствует клиент")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	resp[1] = fmt.Sprint(fs.ModTime().Format("02 Jan 2006"))

	b, err := json.Marshal(&resp)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func processApiOptionsGet(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	logAdd(MESS_INFO, "WEB Запрос опций")

	b, err := json.Marshal(options)
	if err == nil {
		w.Write(b)
		return
	}

	http.Error(w, "", http.StatusBadRequest)
}

func processApiOptionsSave(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	logAdd(MESS_INFO, "WEB Запрос сохранения опций")

	portsmtp := string(r.FormValue("portsmtp"))
	loginsmtp := string(r.FormValue("loginsmtp"))
	passsmtp := string(r.FormValue("passsmtp"))
	loginadmin := string(r.FormValue("loginadmin"))
	passadmin := string(r.FormValue("passadmin"))
	yandex := string(r.FormValue("yandex"))
	version := string(r.FormValue("version"))

	mode, err := strconv.Atoi(string(r.FormValue("mode")))
	if err == nil {
		options.mode = mode
	}

	bufsize, err := strconv.Atoi(string(r.FormValue("bufsize")))
	if err == nil {
		options.SizeBuff = bufsize
	}

	options.PortSMTP = portsmtp
	options.LoginSMTP = loginsmtp
	options.PassSMTP = passsmtp
	options.AdminLogin = loginadmin
	options.AdminPass = passadmin
	options.YandexApiKeyMap = yandex
	options.Version = version

	saveOptions()
	handleOptions(w, r)
}

//общие функции
func checkProfileAuth(w http.ResponseWriter, r *http.Request) *Profile {

	user, pass, ok := r.BasicAuth()

	if ok {
		value, exist := profiles.Load(user)

		if exist {
			if value.(*Profile).Pass == pass {
				//logAdd(MESS_INFO, "WWW Аутентификация успешна " + user + "/"+ r.RemoteAddr)
				return value.(*Profile)
			}
		}
	}

	logAdd(MESS_ERROR, "WWW Аутентификация профиля провалилась "+r.RemoteAddr)
	w.Header().Set("WWW-Authenticate", "Basic")
	http.Error(w, "auth req", http.StatusUnauthorized)
	return nil
}

func checkAdminAuth(w http.ResponseWriter, r *http.Request) bool {

	user, pass, ok := r.BasicAuth()
	if ok {
		if user == options.AdminLogin && pass == options.AdminPass {
			return true
		}
	}

	logAdd(MESS_ERROR, "WWW Аутентификация админки провалилась "+r.RemoteAddr)
	w.Header().Set("WWW-Authenticate", "Basic")
	http.Error(w, "auth req", http.StatusUnauthorized)
	return false
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

func getCounterHour() []string {
	return getCounter(counterData.CounterBytes[:], counterData.CounterConnections[:], counterData.CounterClients[:], 24, int(counterData.currentPos.Hour()))
}

func getCounterDayWeek() []string {
	return getCounter(counterData.CounterDayWeekBytes[:], counterData.CounterDayWeekConnections[:], counterData.CounterDayWeekClients[:], 7, int(counterData.currentPos.Weekday()))
}

func getCounterDay() []string {
	return getCounter(counterData.CounterDayBytes[:], counterData.CounterDayConnections[:], counterData.CounterDayClients[:], 31, int(counterData.currentPos.Day()-1))
}

func getCounterDayYear() []string {
	return getCounter(counterData.CounterDayYearBytes[:], counterData.CounterDayYearConnections[:], counterData.CounterDayYearClients[:], 365, int(counterData.currentPos.YearDay()-1))
}

func getCounterMonth() []string {
	return getCounter(counterData.CounterMonthBytes[:], counterData.CounterMonthConnections[:], counterData.CounterMonthClients[:], 12, int(counterData.currentPos.Month()-1))
}

func pageReplace(e []byte, a string, b string) []byte {
	return bytes.Replace(e, []byte(a), []byte(b), -1)
}

func addMenuAdmin() string {
	out, err := json.Marshal(menuAdmin)
	if err == nil {
		return string(out)
	}

	return ""
}

func addClientsStatisticsAdmin() string {
	var webClientsStatistics []WebClientStatistic

	for _, lists := range clients {
		for _, client := range lists {

			var webClientStatistics WebClientStatistic
			webClientStatistics.Latitude = client.coordinates[0]
			webClientStatistics.Longitude = client.coordinates[1]
			webClientStatistics.Pid = client.Pid
			webClientStatistics.Note = "Версия: " + client.Version + "\n" //todo добавить информацию о профиле

			h, _, err := net.SplitHostPort((*client.Conn).RemoteAddr().String())
			if err == nil {
				webClientStatistics.Ip = h
			}

			webClientsStatistics = append(webClientsStatistics, webClientStatistics)
		}
	}

	out, err := json.Marshal(webClientsStatistics)
	if err == nil {
		return string(out)
	}

	return "null"
}

func addAgentsAdmin() string {
	var webClientsStatistic []WebClientStatistic

	if options.mode == REGULAR {
		var webClientStatistic WebClientStatistic

		webClientStatistic.Latitude = coordinates[0]
		webClientStatistic.Longitude = coordinates[1]

		webClientStatistic.Ip = myIp
		webClientStatistic.Note = "MASTER"

		webClientsStatistic = append(webClientsStatistic, webClientStatistic)
	} else {
		nodes.Range(func(key interface{}, value interface{}) bool {
			curNode := value.(*Node)

			var webClientStatistic WebClientStatistic

			webClientStatistic.Latitude = curNode.coordinates[0]
			webClientStatistic.Longitude = curNode.coordinates[1]

			webClientStatistic.Ip = curNode.Ip
			webClientStatistic.Note = curNode.Name

			webClientsStatistic = append(webClientsStatistic, webClientStatistic)
			return true
		})
	}

	out, err := json.Marshal(webClientsStatistic)
	if err == nil {
		return string(out)
	}

	return "null"
}

func addConnectionsAdmin() string {
	var webConnectionsStatistic []WebConnectionStatistic

	channels.Range(func(key interface{}, value interface{}) bool {
		dConn := value.(*dConn)

		var webConnectionStatistic WebConnectionStatistic

		webConnectionStatistic.Client1.Latitude = dConn.client.coordinates[0]
		webConnectionStatistic.Client1.Longitude = dConn.client.coordinates[1]

		webConnectionStatistic.Client2.Latitude = dConn.server.coordinates[0]
		webConnectionStatistic.Client2.Longitude = dConn.server.coordinates[1]

		if dConn.node != nil {
			webConnectionStatistic.Node.Latitude = (*dConn.node).coordinates[0]
			webConnectionStatistic.Node.Longitude = (*dConn.node).coordinates[1]
		} else {
			webConnectionStatistic.Node.Latitude = coordinates[0]
			webConnectionStatistic.Node.Longitude = coordinates[1]
		}

		webConnectionsStatistic = append(webConnectionsStatistic, webConnectionStatistic)
		return true
	})

	out, err := json.Marshal(webConnectionsStatistic)
	if err == nil {
		return string(out)
	}

	return "null"
}

func addMenuProfile() string {
	out, err := json.Marshal(menuProfile)
	if err == nil {
		return string(out)
	}

	return ""
}
