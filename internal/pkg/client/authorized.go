package client

import (
	"sync"
)

var (
	//держим список авторизованных клиентов в определенный профиль
	// A profile can have multiple connections with the same PID.
	authorized      = map[string]map[*Client]struct{}{}
	authorizedMutex sync.RWMutex
)

func AddAuthorizedClient(email string, client *Client) {
	authorizedMutex.Lock()
	if authorized[email] == nil {
		authorized[email] = make(map[*Client]struct{})
	}
	authorized[email][client] = struct{}{}
	authorizedMutex.Unlock()
}

func DelAuthorizedClient(email string, client *Client) {
	authorizedMutex.Lock()
	delete(authorized[email], client)
	if len(authorized[email]) == 0 {
		delete(authorized, email)
	}
	authorizedMutex.Unlock()

}

func GetAuthorizedClientList(email string) []*Client {
	var list []*Client
	authorizedMutex.RLock()
	for client := range authorized[email] {
		list = append(list, client)
	}
	authorizedMutex.RUnlock()
	return list
}

// возварщает список всех клиентов, для теста
func getContainedAllClientList() []*Client {
	var list []*Client
	authorizedMutex.RLock()
	for _, client := range authorized {
		for item := range client {
			list = append(list, item)
		}
	}
	authorizedMutex.RUnlock()
	return list
}
