package profile

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
	"github.com/vaizmanai/srvisit/internal/pkg/contact"
	"strings"
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

// Data is a profile snapshot without synchronization state.
type Data struct {
	Email    string
	Pass     string
	Contacts *contact.Contact
	Capt     string
	Tel      string
	Logo     string
}

func (p *Profile) Snapshot() Data {
	p.Lock()
	defer p.Unlock()
	return Data{p.Email, p.Pass, contact.Clone(p.Contacts), p.Capt, p.Tel, p.Logo}
}

func (p *Profile) MarshalJSON() ([]byte, error) {
	p.Lock()
	defer p.Unlock()
	return json.Marshal(Data{p.Email, p.Pass, p.Contacts, p.Capt, p.Tel, p.Logo})
}

func (p *Profile) Password() string {
	p.Lock()
	defer p.Unlock()
	return p.Pass
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
	email = strings.ToLower(strings.TrimSpace(email))
	profile, ok := profiles.Load(email)
	if ok {
		return profile.(*Profile)
	}
	return nil
}

func NewProfile(email string) *Profile {
	return NewProfileWithPassword(email, "")
}

func NewProfileWithPassword(email, password string) *Profile {
	email = strings.ToLower(strings.TrimSpace(email))
	mutex.Lock()
	defer mutex.Unlock()

	if GetProfile(email) != nil {
		return nil
	}

	newProfile := Profile{Email: email, Pass: password}
	profiles.Store(newProfile.Email, &newProfile)

	return &newProfile
}

func DelProfile(email string) {
	email = strings.ToLower(strings.TrimSpace(email))
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
	if err := common.LoadFile(common.ProfilesFilename, &list); err != nil {
		log.Errorf("loading profiles: %s", err.Error())
		return
	}

	profiles.Clear()
	for i := 0; i < len(list); i++ {
		list[i].Email = strings.ToLower(strings.TrimSpace(list[i].Email))
		profiles.Store(list[i].Email, &list[i])
	}
}
