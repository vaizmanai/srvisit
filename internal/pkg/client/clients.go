package client

import (
	"crypto/subtle"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
	"github.com/vaizmanai/srvisit/internal/pkg/profile"
	"net"
	"strconv"
	"sync"
)

var (
	//карта подключенных клиентов
	//clients[Pid] = []*Client
	clients      = map[string][]*Client{}
	clientsMutex sync.RWMutex
)

// Client тип для клиента
type Client struct {
	Serial  string
	Pid     string
	Pass    string
	Version string
	Salt    string //for password
	Profile *profile.Profile
	Token   string //for web auth

	Conn *net.Conn
	Code string //for connection

	coordinates [2]float64
	mutex       sync.RWMutex
}

func (client *Client) StoreClient() {
	pid := common.CleanPid(client.Snapshot().Pid)
	clientsMutex.Lock()

	list := clients[pid]
	for _, existing := range list {
		if existing == client {
			clientsMutex.Unlock()
			return
		}
	}
	if list == nil {
		list = make([]*Client, 0)
	}

	common.UpdateCounterClient(true)
	list = append(list, client)
	clients[pid] = list

	clientsMutex.Unlock()
}

func (client *Client) RemoveClient() {
	pid := common.CleanPid(client.Snapshot().Pid)
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
	if len(list) == 0 {
		delete(clients, pid)
	} else {
		clients[pid] = list
	}

	clientsMutex.Unlock()
}

func GetClientsList(pid string) []*Client {
	pid = common.CleanPid(pid)

	clientsMutex.RLock()
	list := append([]*Client{}, clients[pid]...)

	clientsMutex.RUnlock()
	return list
}

// Snapshot returns a copy of the connection state without copying its mutex.
func (client *Client) Snapshot() *Client {
	client.mutex.RLock()
	defer client.mutex.RUnlock()
	return &Client{
		Serial: client.Serial, Pid: client.Pid, Pass: client.Pass,
		Version: client.Version, Salt: client.Salt, Profile: client.Profile,
		Token: client.Token, Conn: client.Conn, Code: client.Code,
		coordinates: client.coordinates,
	}
}

func (client *Client) SetIdentity(serial, pid, salt, token string, conn *net.Conn) {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	client.Serial, client.Pid, client.Salt, client.Token, client.Conn = serial, pid, salt, token, conn
}

func (client *Client) SetVersion(version string) {
	client.mutex.Lock()
	client.Version = version
	client.mutex.Unlock()
}

func (client *Client) SetProfile(p *profile.Profile) *profile.Profile {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	previous := client.Profile
	client.Profile = p
	return previous
}

func (client *Client) MatchesToken(token string) bool {
	client.mutex.RLock()
	defer client.mutex.RUnlock()
	return token != "" && client.Token != "" && subtle.ConstantTimeCompare([]byte(client.Token), []byte(token)) == 1
}

func GetAllClientsList() []*Client {
	resp := make([]*Client, 0)

	clientsMutex.RLock()
	for _, list := range clients {
		for _, client := range list {
			resp = append(resp, client)
		}
	}

	clientsMutex.RUnlock()
	return resp
}

func (client *Client) Coordinates() [2]float64 {
	client.mutex.RLock()
	resp := client.coordinates
	client.mutex.RUnlock()
	return resp
}

func (client *Client) SetCoordinates(coordinate [2]float64) {
	client.mutex.Lock()
	client.coordinates = coordinate
	client.mutex.Unlock()
}

func (client *Client) GreaterVersionThan(version float64) bool {
	client.mutex.RLock()
	defer client.mutex.RUnlock()
	peerVersion, err := strconv.ParseFloat(client.Version, 64)
	if err != nil || peerVersion <= version {
		return false
	}

	return true
}
