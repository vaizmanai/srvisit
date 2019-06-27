package service

import (
    "../common"
    "sync"
)

var (
    //держим список авторизованных клиентов в определенный профиль
    //authorized[email] = map[Client.Pid]*Client
    authorized      map[string]map[string]*Client
    authorizedMutex sync.Mutex
)

func AddAuthorizedClient(email string, client *Client) {
    authorizedMutex.Lock()
    authorized[email][common.CleanPid(client.Pid)] = client
    authorizedMutex.Unlock()
}

func DelAuthorizedClient(email string, client *Client) {
    authorizedMutex.Lock()
    delete(authorized[email], common.CleanPid(client.Pid))
    authorizedMutex.Unlock()

}

func GetListAuthorizedClient(email string) []*Client {
    var list []*Client
    for _, client := range authorized[email] {
        list = append(list, client)
    }
    return list
}
