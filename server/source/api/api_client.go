package api

import (
	"../common"
	"../component/client"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	pausePingSec    = 5
	readDeadlineSec = 10

	wsPing    = 0
	wsMessage = 1
)

//Getting authorized client info
func HandleGetClient(w http.ResponseWriter, r *http.Request, client *client.Client) {
	tmp := *client
	tmp.Pass = "*"
	tmp.Serial = "*"
	tmp.Salt = "*"
	tmp.Code = "*"
	tmp.Token = "*"
	if tmp.Profile != nil {
		tmp.Profile = &*tmp.Profile
		tmp.Profile.Pass = "*"
		//todo надо похерить у контактов реки
	}

	b, err := json.Marshal(tmp)
	if err != nil {
		common.LogAdd(common.MessError, "handleGetClient: "+err.Error())
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}
	w.Write(b)
}

type websocketMessage struct {
	Type int
	Data string
}

type chatMessage struct {
	Pid  string
	Id   int
	Text string
}

type wsClient struct {
	mutex  sync.RWMutex
	ws     *websocket.Conn
	client *client.Client
}

var (
	//wsClients[Pid] = []*wsClient
	wsClients      map[string][]*wsClient
	wsClientsMutex sync.RWMutex
)

func init() {
	wsClients = make(map[string][]*wsClient, 0)
}

func (curWsClient *wsClient) storeWsClient() {
	pid := common.CleanPid(curWsClient.client.Pid)
	wsClientsMutex.Lock()

	list := wsClients[pid]
	if list == nil {
		list = make([]*wsClient, 0)
	}

	list = append(list, curWsClient)
	wsClients[pid] = list

	wsClientsMutex.Unlock()
}

func (curWsClient *wsClient) removeWsClient() {
	pid := common.CleanPid(curWsClient.client.Pid)
	wsClientsMutex.Lock()

	list := wsClients[pid]
	if list != nil {
		for i := 0; i < len(list); {
			if list[i] == curWsClient {
				if len(list) == 1 {
					list = make([]*wsClient, 0)
					break
				}

				list = append(list[:i], list[i+1:]...)
				continue
			}
			i++
		}
	}
	wsClients[pid] = list

	wsClientsMutex.Unlock()
}

func (curWsClient *wsClient) SendJSON(v interface{}) error {
	curWsClient.mutex.Lock()
	err := curWsClient.ws.WriteJSON(v)
	curWsClient.mutex.Unlock()
	return err
}

func GetWsClient(pid string) []*wsClient {
	pid = common.CleanPid(pid)

	wsClientsMutex.RLock()
	list := wsClients[pid]
	if list == nil {
		list = make([]*wsClient, 0)
	}

	wsClientsMutex.RUnlock()
	return list
}

func HandleChatWS(ws *websocket.Conn, client *client.Client) {
	common.LogAdd(common.MessInfo, "web ws connected")
	if ws == nil {
		common.LogAdd(common.MessError, "web ws is null")
		return
	}

	curWsClient := &wsClient{ws: ws, client: client}
	curWsClient.storeWsClient()

	go func() {
		m := websocketMessage{
			Type: wsPing,
			Data: "",
		}

		for {
			time.Sleep(time.Second * pausePingSec)
			if ws == nil {
				common.LogAdd(common.MessError, "error writing json: closed connection")
				curWsClient.removeWsClient()
				return
			}

			err := curWsClient.SendJSON(m)

			if err != nil {
				common.LogAdd(common.MessError, "error writing json: "+fmt.Sprint(err))
				curWsClient.removeWsClient()
				return
			}
		}
	}()

	for {
		m := websocketMessage{}

		err := ws.SetReadDeadline(time.Now().Add(time.Second * readDeadlineSec))
		if err != nil {
			common.LogAdd(common.MessError, "error reading json: "+fmt.Sprint(err))
			curWsClient.removeWsClient()
			break
		}

		err = ws.ReadJSON(&m)
		if err != nil {
			common.LogAdd(common.MessError, "error reading json: "+fmt.Sprint(err))
			curWsClient.removeWsClient()
			break
		}

		common.LogAdd(common.MessDetail, fmt.Sprint(m))

		if m.Type == wsPing {

		} else if m.Type == wsMessage {
			chat := chatMessage{}
			err = json.Unmarshal([]byte(m.Data), &chat)
			if err != nil {
				continue
			}

			chat.Text = common.DecodeB64(chat.Text)

			chat.Text = strings.Replace(chat.Text, "<", "[", -1)
			chat.Text = strings.Replace(chat.Text, ">", "]", -1)

			common.LogAdd(common.MessFull, "chat "+client.Pid+" -> "+chat.Pid+": "+chat.Text)

			chat.Text = common.EncodeB64(chat.Text)
			b, _ := json.Marshal(chat)
			m.Data = string(b)

			list := GetWsClient(chat.Pid)
			for i := 0; i < len(list); i++ {
				list[i].SendJSON(m)
			}
		}
	}

	common.LogAdd(common.MessInfo, "web ws disconnected")
	curWsClient.removeWsClient()
}
