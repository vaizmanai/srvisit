package client

import (
	"../../common"
	. "../profile"
	"sync"
)

var (
	//держим список онлайн клиентов которые присутствуют в профилях - используем для оперативной отправки статуса
	//contained[Client.Pid] = map[email]*Profile
	contained      map[string]map[string]*Profile
	containedMutex sync.RWMutex
)

func init() {
	contained = make(map[string]map[string]*Profile)
}

func AddContainedProfile(pid string, profile *Profile) {
	pid = common.CleanPid(pid)

	containedMutex.Lock()
	if contained[pid] == nil {
		contained[pid] = make(map[string]*Profile)
	}
	contained[pid][profile.Email] = profile
	containedMutex.Unlock()
}

func DelContainedProfile(pid string, profile *Profile) {
	pid = common.CleanPid(pid)

	containedMutex.Lock()
	if contained[pid] == nil {
		contained[pid] = make(map[string]*Profile)
	}
	delete(contained[pid], profile.Email)
	containedMutex.Unlock()

}

//возварщает список профилей где есть указанный pid
func GetContainedProfileList(pid string) []*Profile {
	pid = common.CleanPid(pid)
	containedMutex.RLock()
	var list []*Profile
	for _, profile := range contained[pid] {
		list = append(list, profile)
	}
	containedMutex.RUnlock()
	return list
}

//возварщает список всех профилей, для теста
func getContainedAllProfileList() []*Profile {
	containedMutex.RLock()
	var list []*Profile
	for _, profile := range contained {
		for _, item := range profile {
			list = append(list, item)
		}
	}
	containedMutex.RUnlock()
	return list
}
