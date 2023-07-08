package profile

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"os"
	"srvisit/internal/pkg/common"
	"srvisit/internal/pkg/contact"
	"sync"
)

var (
	profiles sync.Map
	mutex    sync.Mutex
)

// Profile is base structure for storing contacts
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

// todo избавиться
func (profile *Profile) Lock() {
	profile.contactsMutex.Lock()
}

// todo избавиться
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
	mutex.Lock()
	defer mutex.Unlock()

	list := GetProfileList()

	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		log.Errorf("не удалось сохранить профили: %s", err.Error())
		return
	}

	f, err := os.Create(common.ProfilesFilename + ".tmp")
	if err != nil {
		log.Errorf("не удалось сохранить профили: %s", err.Error())
		return
	}

	n, err := f.Write(b)
	if n == len(b) && err == nil {
		err = f.Close()
		if err != nil {
			log.Errorf("ошибка при сохранении профилей: %s", err.Error())
		}

		err = os.Remove(common.ProfilesFilename)
		if err != nil {
			//если это первое сохранение, то файла может ещё и не быть который мы хотим удалить
			//log.Errorf("ошибка при сохранении профилей: %s", err.Error())
		}

		err = os.Rename(common.ProfilesFilename+".tmp", common.ProfilesFilename)
		if err != nil {
			log.Errorf("ошибка при сохранении профилей: %s", err.Error())
		}
	} else {
		log.Errorf("не удалось сохранить профили: %s", err.Error())

		err = f.Close()
		if err != nil {
			log.Errorf("ошибка при сохранении профилей: %s", err.Error())
		}
	}
}

func LoadProfiles() {
	var list []Profile
	profiles = sync.Map{}

	b, err := os.ReadFile(common.ProfilesFilename)
	if err != nil {
		log.Errorf("не получилось загрузить профили: %s", err.Error())
		return
	}

	err = json.Unmarshal(b, &list)
	if err != nil {
		log.Errorf("не получилось загрузить профили: %s", err.Error())
		return
	}

	for i := 0; i < len(list); i++ {
		profiles.Store(list[i].Email, &list[i])
	}
}
