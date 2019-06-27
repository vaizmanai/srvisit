package client

import (
    "../../common"
    "sync"
)

var (
    //держим список авторизованных клиентов в определенный профиль
    //authorized[email] = map[Client.Pid]*Client
    authorized      map[string]map[string]*Client
    authorizedMutex sync.Mutex
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
    for _, client := range authorized[email] {
        list = append(list, client)
    }
    return list
}
