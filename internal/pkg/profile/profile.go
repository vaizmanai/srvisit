package profile

import (
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
	"github.com/vaizmanai/srvisit/internal/pkg/contact"
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

	if err := common.SaveFile(common.ProfilesFilename, GetProfileList()); err != nil {
		log.Errorf("saving profiles: %s", err.Error())
	}
}

func LoadProfiles() {
	mutex.Lock()
	defer mutex.Unlock()

	var list []Profile
	profiles = sync.Map{}

	if err := common.LoadFile(common.ProfilesFilename, &list); err != nil {
		log.Errorf("loading profiles: %s", err.Error())
	}

	for i := 0; i < len(list); i++ {
		profiles.Store(list[i].Email, &list[i])
	}
}
