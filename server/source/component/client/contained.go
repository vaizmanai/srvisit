package client

import (
    "../../common"
    . "../profile"
    "sync"
)

var (
    //держим список онлайн клиентов которые присутствуют в профилях - используем для оперативной отправки статуса
    //contained[Client.Pid] = map[email]*Profile
    contained     map[string]map[string]*Profile
    profilesMutex sync.Mutex
)

func init() {
    contained = make(map[string]map[string]*Profile)
}

func AddContainedProfile(pid string, profile *Profile) {
    pid = common.CleanPid(pid)

    profilesMutex.Lock()
    if contained[pid] == nil {
        contained[pid] = make(map[string]*Profile)
    }
    contained[pid][profile.Email] = profile
    profilesMutex.Unlock()
}

func DelContainedProfile(pid string, profile *Profile) {
    pid = common.CleanPid(pid)

    profilesMutex.Lock()
    if contained[pid] == nil {
        contained[pid] = make(map[string]*Profile)
    }
    delete(contained[pid], profile.Email)
    profilesMutex.Unlock()

}

func GetContainedProfileList(pid string) []*Profile {
    pid = common.CleanPid(pid)

    var list []*Profile
    for _, profile := range contained[pid] {
        list = append(list, profile)
    }
    return list
}
