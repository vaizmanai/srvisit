package profile

import (
	"../../common"
	"../contact"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
)

var (
	profiles sync.Map
	mutex    sync.Mutex
)

//Profile is base structure for storing contacts
type Profile struct {
	Email string
	Pass  string

	Contacts      *contact.Contact
	contactsMutex sync.Mutex

	//всякая информация
	Capt string
	Tel  string
	Logo string
}

//todo избавиться
func (profile *Profile) Lock() {
	profile.contactsMutex.Lock()
}

//todo избавиться
func (profile *Profile) Unlock() {
	profile.contactsMutex.Unlock()
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

func GetProfileList() []*Profile {
	list := make([]*Profile, 0)
	profiles.Range(func(key interface{}, value interface{}) bool {
		list = append(list, value.(*Profile))
		return true
	})
	return list
}

func SaveProfiles() {
	list := GetProfileList()

	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		common.LogAdd(common.MessError, "Не удалось сохранить профили: "+fmt.Sprint(err))
		return
	}

	f, err := os.Create(common.ProfilesFilename + ".tmp")
	if err != nil {
		common.LogAdd(common.MessError, "Не удалось сохранить профили: "+fmt.Sprint(err))
		return
	}

	n, err := f.Write(b)
	if n == len(b) && err == nil {
		err = f.Close()
		if err != nil {
			common.LogAdd(common.MessError, "Ошибка при сохранении профилей: "+fmt.Sprint(err))
		}

		err = os.Remove(common.ProfilesFilename)
		if err != nil {
			//если это первое сохранение, то файла может ещё и не быть который мы хотим удалить
			//common.LogAdd(common.MessError, "Ошибка при сохранении профилей: "+fmt.Sprint(err))
		}

		err = os.Rename(common.ProfilesFilename+".tmp", common.ProfilesFilename)
		if err != nil {
			common.LogAdd(common.MessError, "Ошибка при сохранении профилей: "+fmt.Sprint(err))
		}
	} else {
		common.LogAdd(common.MessError, "Не удалось сохранить профили: "+fmt.Sprint(err))

		err = f.Close()
		if err != nil {
			common.LogAdd(common.MessError, "Ошибка при сохранении профилей: "+fmt.Sprint(err))
		}
	}
}

func LoadProfiles() {
	var list []Profile
	profiles = sync.Map{}

	f, err := os.Open(common.ProfilesFilename)
	if err != nil {
		common.LogAdd(common.MessError, "Не получилось загрузить профилей: "+fmt.Sprint(err))
		return
	}
	defer func() {
		err = f.Close()
		if err != nil {
			common.LogAdd(common.MessError, "Ошибка при загрузки профилей: "+fmt.Sprint(err))
		}
	}()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		common.LogAdd(common.MessError, "Не получилось загрузить профили: "+fmt.Sprint(err))
		return
	}

	err = json.Unmarshal(b, &list)
	if err != nil {
		common.LogAdd(common.MessError, "Не получилось загрузить профили: "+fmt.Sprint(err))
		return
	}

	for i := 0; i < len(list); i++ {
		profiles.Store(list[i].Email, &list[i])
	}
}
