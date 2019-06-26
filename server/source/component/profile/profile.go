package profile

import (
	"../../common"
	. "../contact"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
)

var (
	//карта учеток
	profiles sync.Map
	mutex    sync.Mutex
	//	profiles = make(map[string]int)
)

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

//todo избавиться
func GetProfiles() (*sync.Map) {
	return &profiles
}

//todo избавиться
func (profile *Profile) GetClients() (*sync.Map) {
	return &(*profile).clients
}

func (profile *Profile) Lock() {
	profile.mutex.Lock()
}

func (profile *Profile) Unlock() {
	profile.mutex.Unlock()
}

func GetProfile(email string) *Profile {
	profile, ok := profiles.Load(email)
	if ok {
		return profile.(*Profile)
	}
	return nil
}

func NewProfile(email string) *Profile {
	mutex.Lock()
	defer mutex.Unlock()

	if GetProfile(email) != nil {
		return nil
	}

	newProfile := Profile{Email: email}
	profiles.Store(newProfile.Email, &newProfile)

	return &newProfile
}

func DelProfile(email string) {
	mutex.Lock()
	defer mutex.Unlock()

	profiles.Delete(email)
}

func GetProfileList() []Profile{
	list := make([]Profile, 0)
	profiles.Range(func(key interface{}, value interface{}) bool {
		list = append(list, *value.(*Profile))
		return true
	})
	return list
}

func SaveProfiles() {
	var list []*Profile

	profiles.Range(func(key interface{}, value interface{}) bool {
		profile := value.(*Profile)
		list = append(list, profile)
		return true
	})

	b, err := json.Marshal(list)
	if err == nil {
		f, err := os.Create(common.ProfilesFilename + ".tmp")
		if err == nil {
			n, err := f.Write(b)
			if n == len(b) && err == nil {
				f.Close()

				os.Remove(common.ProfilesFilename)
				os.Rename(common.ProfilesFilename+".tmp", common.ProfilesFilename)
			} else {
				f.Close()
				common.LogAdd(common.MessError, "Не удалось сохранить профили: "+fmt.Sprint(err))
			}
		} else {
			common.LogAdd(common.MessError, "Не удалось сохранить профили: "+fmt.Sprint(err))
		}
	} else {
		common.LogAdd(common.MessError, "Не удалось сохранить профили: "+fmt.Sprint(err))
	}
}

func LoadProfiles() {
	var list []Profile

	f, err := os.Open(common.ProfilesFilename)
	defer f.Close()
	if err == nil {
		b, err := ioutil.ReadAll(f)
		if err == nil {
			err = json.Unmarshal(b, &list)
			if err == nil {
				for i := 0; i < len(list); i++ {
					profiles.Store(list[i].Email, &list[i])
				}
			} else {
				common.LogAdd(common.MessError, "Не получилось загрузить профили: "+fmt.Sprint(err))
			}
		} else {
			common.LogAdd(common.MessError, "Не получилось загрузить профили: "+fmt.Sprint(err))
		}
	} else {
		common.LogAdd(common.MessError, "Не получилось загрузить профили: "+fmt.Sprint(err))
	}
}
