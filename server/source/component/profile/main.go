package profile

import (
	. "../contact"
	"sync"
)

//var (
//	//карта учеток
//	//profiles sync.Map
//	profiles = make(map[string]int)
//)

//тип для профиля
type Profile struct {
	Email string
	Pass  string

	Contacts *Contact
	mutex    sync.Mutex

	clients sync.Map //клиенты которые авторизовались в этот профиль(используем для отправки им информации о статусе или изменений контактов)

	//всякая информация
	Capt string
	Tel  string
	Logo string
}


func (profile *Profile) GetClients() (*sync.Map) {
	return &(*profile).clients
}

func (profile *Profile) Lock() {
	profile.mutex.Lock()
}

func (profile *Profile) Unlock() {
	profile.mutex.Unlock()
}