package client

import (
	"srvisit/internal/pkg/common"
	"srvisit/internal/pkg/profile"
	"sync"
)

var (
	//держим список онлайн клиентов которые присутствуют в профилях - используем для оперативной отправки статуса
	//contained[Client.Pid] = map[email]*Profile
	contained      = map[string]map[string]*profile.Profile{}
	containedMutex sync.RWMutex
)

func AddContainedProfile(pid string, p *profile.Profile) {
	pid = common.CleanPid(pid)

	containedMutex.Lock()
	if contained[pid] == nil {
		contained[pid] = make(map[string]*profile.Profile)
	}
	contained[pid][p.Email] = p
	containedMutex.Unlock()
}

func DelContainedProfile(pid string, p *profile.Profile) {
	pid = common.CleanPid(pid)

	containedMutex.Lock()
	if contained[pid] == nil {
		contained[pid] = make(map[string]*profile.Profile)
	}
	delete(contained[pid], p.Email)
	containedMutex.Unlock()
}

// GetContainedProfileList возвращает список профилей где есть указанный pid
func GetContainedProfileList(pid string) []*profile.Profile {
	pid = common.CleanPid(pid)
	containedMutex.RLock()
	var list []*profile.Profile
	for _, p := range contained[pid] {
		list = append(list, p)
	}
	containedMutex.RUnlock()
	return list
}

// getContainedAllProfileList возвращает список всех профилей, для теста
func getContainedAllProfileList() []*profile.Profile {
	containedMutex.RLock()
	var list []*profile.Profile
	for _, p := range contained {
		for _, item := range p {
			list = append(list, item)
		}
	}
	containedMutex.RUnlock()
	return list
}
