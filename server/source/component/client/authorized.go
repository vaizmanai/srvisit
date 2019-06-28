package client

import (
	"../../common"
	"sync"
)

var (
	//держим список авторизованных клиентов в определенный профиль
	//authorized[email] = map[Client.Pid]*Client
	authorized      map[string]map[string]*Client
	authorizedMutex sync.RWMutex
)

func init() {
	authorized = make(map[string]map[string]*Client)
}

func AddAuthorizedClient(email string, client *Client) {
	authorizedMutex.Lock()
	if authorized[email] == nil {
		authorized[email] = make(map[string]*Client)
	}
	authorized[email][common.CleanPid(client.Pid)] = client
	authorizedMutex.Unlock()
}

func DelAuthorizedClient(email string, client *Client) {
	authorizedMutex.Lock()
	if authorized[email] == nil {
		authorized[email] = make(map[string]*Client)
	}
	delete(authorized[email], common.CleanPid(client.Pid))
	authorizedMutex.Unlock()

}

func GetAuthorizedClientList(email string) []*Client {
	var list []*Client
	authorizedMutex.RLock()
	for _, client := range authorized[email] {
		list = append(list, client)
	}
	authorizedMutex.RUnlock()
	return list
}

//возварщает список всех клиентов, для теста
func getContainedAllClientList() []*Client {
	var list []*Client
	authorizedMutex.RLock()
	for _, client := range authorized {
		for _, item := range client {
			list = append(list, item)
		}
	}
	authorizedMutex.RUnlock()
	return list
}
