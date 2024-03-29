package processor

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/api"
	"github.com/vaizmanai/srvisit/internal/pkg/client"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
	"github.com/vaizmanai/srvisit/internal/pkg/contact"
	"github.com/vaizmanai/srvisit/internal/pkg/profile"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

var (
	//функции для обработки web api
	processingWeb = []ProcessingWeb{
		{"defaultvnc", processApiDefaultVnc},
		{"listvnc", processApiListVnc},
		{"getlog", processApiGetLog},
		{"clearlog", processApiClearLog},
		{"profile_save", processApiProfileSave},
		{"profile_get", processApiProfileGet},
		{"save_options", processApiSaveOptions},
		{"options_save", processApiOptionsSave},
		{"reload", processApiReload},
		{"reopen", processApiReopen},
		{"options_get", processApiOptionsGet},
		{"version", processApiVersion}}

	//меню веб интерфейса админки
	menuAdmin = []itemMenu{
		{"Логи", "/admin/logs"},
		{"Настройки", "/admin/options"},
		{"Ресурсы", "/admin/resources"},
		{"Статистика", "/admin/statistics"},
		{"reVisit", "/resource/reVisit.exe"}}

	//меню веб интерфейса профиля
	menuProfile = []itemMenu{
		{"Профиль", "/profile/my"},
		{"reVisit", "/resource/reVisit.exe"}}
)

// меню для веба
type itemMenu struct {
	Capt string
	Link string
}

type WebClientStatistic struct {
	Latitude  float64
	Longitude float64
	Pid       string
	Ip        string
	Note      string
}

type WebConnectionStatistic struct {
	Client1 WebClientStatistic
	Client2 WebClientStatistic

	Node WebClientStatistic
}

// обработчик для веб запроса
type ProcessingWeb struct {
	Make       string
	Processing func(w http.ResponseWriter, r *http.Request)
}

// хэндлеры для профиля
func handleProfileWelcome(w http.ResponseWriter, r *http.Request) {

	file, _ := os.Open("resource/profile/welcome.html")
	body, err := io.ReadAll(file)
	if err == nil {
		_ = file.Close()

		body = pageReplace(body, "$menu", addMenuProfile())
		_, _ = w.Write(body)
		return
	}

}

func handleProfileMy(w http.ResponseWriter, r *http.Request) {
	curProfile := checkProfileAuth(w, r)

	if curProfile == nil {
		return
	}

	file, _ := os.Open("resource/profile/my.html")
	body, err := io.ReadAll(file)
	if err == nil {
		_ = file.Close()

		body = pageReplace(body, "$menu", addMenuProfile())
		_, _ = w.Write(body)
		return
	}
}

// хэндлеры для админки
func handleWelcome(w http.ResponseWriter, r *http.Request) {

	file, _ := os.Open("resource/admin/welcome.html")
	body, err := io.ReadAll(file)
	if err == nil {
		_ = file.Close()

		body = pageReplace(body, "$menu", addMenuAdmin())
		_, _ = w.Write(body)
		return
	}

}

func handleResources(w http.ResponseWriter, r *http.Request) {

	if !checkAdminAuth(w, r) {
		return
	}

	connectionsString := ""

	var buf1 string
	if common.Options.Mode == common.ModeMaster {
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

	for _, c := range client.GetAllClientsList() {
		if c.Profile == nil {
			buf1 = "no auth"
		} else {
			buf1 = c.Profile.Email
		}

		connectionsString = connectionsString + fmt.Sprintln(c.Pid, c.Serial, c.Version, (*c.Conn).RemoteAddr(), buf1)

		for _, p := range client.GetContainedProfileList(c.Pid) {
			var capt string
			c := contact.GetContactByPid(p.Contacts, common.CleanPid(c.Pid)) //todo потом убрать, лишние итерации не сильно нам интересны
			if c != nil {
				capt = fmt.Sprint("/ ", c.Caption)
			}

			connectionsString = connectionsString + fmt.Sprintln("\t ->", p.Email, capt)
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

	for _, p := range profile.GetProfileList() {
		connectionsString = connectionsString + fmt.Sprintln(p.Email) //(*value.(*Profile)).Pass)

		for _, c := range client.GetAuthorizedClientList(p.Email) {
			connectionsString = connectionsString + fmt.Sprintln("\t", "<- "+c.Pid)
		}
	}

	connectionsString = connectionsString + "</pre></div>"

	file, _ := os.Open("resource/admin/resources.html")
	body, err := io.ReadAll(file)
	if err == nil {
		_ = file.Close()

		body = pageReplace(body, "$menu", addMenuAdmin())
		body = pageReplace(body, "$connections", connectionsString)
		_, _ = w.Write(body)
		return
	}

}

func handleStatistics(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	file, _ := os.Open("resource/admin/statistics.html")
	body, err := io.ReadAll(file)
	if err == nil {
		_ = file.Close()

		body = pageReplace(body, "$menu", addMenuAdmin())
		body = pageReplace(body, "$clients", addClientsStatisticsAdmin())
		body = pageReplace(body, "$connections", addConnectionsAdmin())
		body = pageReplace(body, "$agents", addAgentsAdmin())

		charts := common.GetCounterHour()
		body = pageReplace(body, "$headers01", charts[0]) //по часам
		body = pageReplace(body, "$values01", charts[1])
		body = pageReplace(body, "$values02", charts[2])
		body = pageReplace(body, "$values21", charts[3])

		charts = common.GetCounterDayWeek()
		body = pageReplace(body, "$headers02", charts[0]) //по дням недели
		body = pageReplace(body, "$values03", charts[1])
		body = pageReplace(body, "$values04", charts[2])
		body = pageReplace(body, "$values22", charts[3])

		charts = common.GetCounterDay()
		body = pageReplace(body, "$headers03", charts[0]) //по дням месяца
		body = pageReplace(body, "$values05", charts[1])
		body = pageReplace(body, "$values06", charts[2])
		body = pageReplace(body, "$values23", charts[3])

		charts = common.GetCounterDayYear()
		body = pageReplace(body, "$headers04", charts[0]) //по дням года
		body = pageReplace(body, "$values07", charts[1])
		body = pageReplace(body, "$values08", charts[2])
		body = pageReplace(body, "$values24", charts[3])

		charts = common.GetCounterMonth()
		body = pageReplace(body, "$headers05", charts[0]) //по месяцам
		body = pageReplace(body, "$values09", charts[1])
		body = pageReplace(body, "$values10", charts[2])
		body = pageReplace(body, "$values25", charts[3])

		_, _ = w.Write(body)
		return
	}
}

func handleOptions(w http.ResponseWriter, r *http.Request) {

	if !checkAdminAuth(w, r) {
		return
	}

	file, _ := os.Open("resource/admin/Options.html")
	body, err := io.ReadAll(file)
	if err == nil {
		_ = file.Close()

		body = pageReplace(body, "$menu", addMenuAdmin())
		//body = pageReplace(body, "$logs", logsString)
		_, _ = w.Write(body)
		return
	}

}

func handleLogs(w http.ResponseWriter, r *http.Request) {

	if !checkAdminAuth(w, r) {
		return
	}

	file, _ := os.Open("resource/admin/logs.html")
	body, err := io.ReadAll(file)
	if err == nil {
		_ = file.Close()

		body = pageReplace(body, "$menu", addMenuAdmin())
		//body = pageReplace(body, "$logs", logsString)
		_, _ = w.Write(body)
		return
	}

}

// ресурсы и api
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
				log.Infof("WEB Нет обработчика для сообщения")
				time.Sleep(time.Millisecond * common.WaitIdle)
			}
			return
		}
	}

	time.Sleep(time.Millisecond * common.WaitIdle)
	log.Errorf("WEB Неизвестное сообщение")
	http.Error(w, "bad request", http.StatusBadRequest)
}

// раскрытие api
func processApiDefaultVnc(w http.ResponseWriter, r *http.Request) {
	log.Infof("WEB Запрос vnc версии по-умолчанию")

	if len(arrayVnc) < defaultVnc {
		buff, err := json.Marshal(arrayVnc[defaultVnc])
		if err != nil {
			log.Errorf("WEB Не получилось отправить версию VNC")
			return
		}
		_, _ = w.Write(buff)
		return
	}
	http.Error(w, "vnc is not prepared", http.StatusNotAcceptable)
}

func processApiListVnc(w http.ResponseWriter, r *http.Request) {
	log.Infof("WEB Запрос списка vnc")

	buff, err := json.Marshal(arrayVnc)
	if err != nil {
		log.Errorf("WEB Не получилось отправить список VNC")
		return
	}
	_, _ = w.Write(buff)
}

func processApiGetLog(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	api.HandleGetLog(w, r)
}

func processApiClearLog(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	log.Infof("WEB Запрос очистки log")
	common.CloseLogFile()
	http.Redirect(w, r, "/admin/logs", http.StatusTemporaryRedirect)
}

func processApiProfileSave(w http.ResponseWriter, r *http.Request) {
	curProfile := checkProfileAuth(w, r)
	if curProfile == nil {
		return
	}

	log.Infof("WEB Запрос сохранения профиля " + curProfile.Email)

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

	log.Infof("WEB Запрос информации профиля " + curProfile.Email)

	newProfile := *curProfile
	newProfile.Pass = "*****"
	b, err := json.Marshal(&newProfile)
	if err == nil {
		_, _ = w.Write(b)
		return
	}

	http.Error(w, "", http.StatusBadRequest)
}

func processApiSaveOptions(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	log.Infof("WEB Запрос сохранения опций")

	common.SaveOptions()

	handleOptions(w, r)
}

func processApiReload(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	log.Infof("WEB Запрос на перезапуск сервера")

	//todo перезапуск
	w.WriteHeader(http.StatusOK)
}

func processApiReopen(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	log.Infof("WEB Запрос на чтение списка VNC")

	LoadVNCList()
	w.WriteHeader(http.StatusOK)
}

func processApiVersion(w http.ResponseWriter, r *http.Request) {
	log.Infof("WEB Запрос актуальной версии")

	var resp [2]string
	resp[0] = common.Options.Version
	fs, err := os.Stat("resource/reVisit.exe")
	if err != nil {
		log.Errorf("WEB Отсутствует клиент")
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
	_, _ = w.Write(b)
}

func processApiOptionsGet(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	log.Infof("WEB Запрос опций")

	b, err := json.Marshal(common.Options)
	if err == nil {
		_, _ = w.Write(b)
		return
	}

	http.Error(w, "", http.StatusBadRequest)
}

func processApiOptionsSave(w http.ResponseWriter, r *http.Request) {
	if !checkAdminAuth(w, r) {
		return
	}

	log.Infof("WEB Запрос сохранения опций")

	portsmtp := r.FormValue("portsmtp")
	loginsmtp := r.FormValue("loginsmtp")
	passsmtp := r.FormValue("passsmtp")
	loginadmin := r.FormValue("loginadmin")
	passadmin := r.FormValue("passadmin")
	yandex := r.FormValue("yandex")
	version := r.FormValue("version")

	mode, err := strconv.Atoi(r.FormValue("mode"))
	if err == nil {
		common.Options.Mode = mode
	}

	bufsize, err := strconv.Atoi(r.FormValue("bufsize"))
	if err == nil {
		common.Options.SizeBuff = bufsize
	}

	common.Options.PortSMTP = portsmtp
	common.Options.LoginSMTP = loginsmtp
	common.Options.PassSMTP = passsmtp
	common.Options.AdminLogin = loginadmin
	common.Options.AdminPass = passadmin
	common.Options.YandexApiKeyMap = yandex
	common.Options.Version = version

	common.SaveOptions()
	handleOptions(w, r)
}

// общие функции
func checkProfileAuth(w http.ResponseWriter, r *http.Request) *profile.Profile {

	user, pass, ok := r.BasicAuth()

	if ok {
		if p := profile.GetProfile(user); p != nil {
			if p.Pass == pass {
				//log.Infof("WWW Аутентификация успешна " + user + "/"+ r.RemoteAddr)
				return p
			}
		}
	}

	log.Errorf("WWW Аутентификация профиля провалилась " + r.RemoteAddr)
	w.Header().Set("WWW-Authenticate", "Basic")
	http.Error(w, "auth req", http.StatusUnauthorized)
	return nil
}

func checkAdminAuth(w http.ResponseWriter, r *http.Request) bool {

	user, pass, ok := r.BasicAuth()
	if ok {
		if user == common.Options.AdminLogin && pass == common.Options.AdminPass {
			return true
		}
	}

	log.Errorf("WWW Аутентификация провалилась " + r.RemoteAddr)
	w.Header().Set("WWW-Authenticate", "Basic")
	http.Error(w, "auth req", http.StatusUnauthorized)
	return false
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

	for _, c := range client.GetAllClientsList() {
		var webClientStatistics WebClientStatistic
		webClientStatistics.Latitude = c.Coordinates()[0]
		webClientStatistics.Longitude = c.Coordinates()[1]
		webClientStatistics.Pid = c.Pid
		webClientStatistics.Note = "Версия: " + c.Version + "\n" //todo добавить информацию о профиле

		h, _, err := net.SplitHostPort((*c.Conn).RemoteAddr().String())
		if err == nil {
			webClientStatistics.Ip = h
		}

		webClientsStatistics = append(webClientsStatistics, webClientStatistics)
	}

	if out, err := json.Marshal(webClientsStatistics); err != nil {
		return "null"
	} else {
		return string(out)
	}
}

func addAgentsAdmin() string {
	var webClientsStatistic []WebClientStatistic

	if common.Options.Mode == common.ModeRegular {
		var webClientStatistic WebClientStatistic

		webClientStatistic.Latitude = coordinates[0]
		webClientStatistic.Longitude = coordinates[1]

		webClientStatistic.Ip = myIp
		webClientStatistic.Note = "ModeMaster"

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

		webConnectionStatistic.Client1.Latitude = dConn.client.Coordinates()[0]
		webConnectionStatistic.Client1.Longitude = dConn.client.Coordinates()[1]

		webConnectionStatistic.Client2.Latitude = dConn.server.Coordinates()[0]
		webConnectionStatistic.Client2.Longitude = dConn.server.Coordinates()[1]

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
