package client

import (
	"../../common"
	. "../profile"
	"net"
	"sync"
)

var (
	//карта подключенных клиентов
	//clients[Pid] = []*Client
	clients      map[string][]*Client
	clientsMutex sync.Mutex
)

//тип для клиента
type Client struct {
	Serial  string
	Pid     string
	Pass    string
	Version string
	Salt    string //for password
	Profile *Profile
	Token   string //for web auth

	Conn *net.Conn
	Code string //for connection

	coordinates [2]float64
}

func init() {
	clients = make(map[string][]*Client, 0)
}

func (client *Client) StoreClient() {
	pid := common.CleanPid(client.Pid)
	clientsMutex.Lock()

	list := clients[pid]
	if list == nil {
		list = make([]*Client, 0)
	}

	common.UpdateCounterClient(true)
	list = append(list, client)
	clients[pid] = list

	clientsMutex.Unlock()
}

func (client *Client) RemoveClient() {
	pid := common.CleanPid(client.Pid)
	clientsMutex.Lock()

	list := clients[pid]
	if list != nil {
		for i := 0; i < len(list); {
			if list[i] == client {
				if len(list) == 1 {
					common.UpdateCounterClient(false)
					list = make([]*Client, 0)
					break
				}
				common.UpdateCounterClient(false)
				list[i] = list[len(list)-1]
				list = list[:len(list)-1]
				continue
			}
			i++
		}
	}
	clients[pid] = list

	clientsMutex.Unlock()
}

func GetClientsList(pid string) []*Client {
	pid = common.CleanPid(pid)

	list := clients[pid]
	if list == nil {
		list = make([]*Client, 0)
	}

	return list
}

func GetAllClientsList() []*Client {
	resp := make([]*Client, 0)

	for _, list := range clients {
		for _, client := range list {
			resp = append(resp, client)
		}
	}

	return resp
}

func (client *Client) Coordinates() [2]float64 {
	return client.coordinates
}

func (client *Client) SetCoordinates(coordinate [2]float64) {
	client.coordinates = coordinate
}
