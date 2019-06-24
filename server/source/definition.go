package main

var (
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



//меню для веба
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
