package api

import (
	"../common"
	"../component/client"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"net/http"
	"os"
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

type FileDescription struct {
	Name  string
	IsDir bool
	Mode  os.FileMode
	Time  time.Time
	Size  int64
}

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

//Getting authorized client's files
func HandleGetFiles(w http.ResponseWriter, r *http.Request, client *client.Client) {
	if client.Profile == nil {
		common.LogAdd(common.MessError, "HandleGetFiles: profile is null")
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}

	fileName := "/"
	p := strings.Split(r.RequestURI, "/v2/api/files")
	if len(p) == 2 {
		fileName = p[1]
	}

	fs, err := os.Stat("files/" + client.Profile.Email + fileName)
	if err != nil {
		common.LogAdd(common.MessError, "HandleGetFiles: "+err.Error())
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}

	if !fs.IsDir() {
		f, err := os.Open("files/" + client.Profile.Email + fileName)
		if err != nil {
			common.LogAdd(common.MessError, "HandleGetFiles: "+err.Error())
			http.Error(w, "couldn't service this request", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		w.Header().Set("Content-Type", "multipart/mixed")
		w.Header().Set("Content-Disposition", "attachment; filename="+fs.Name())
		w.Header().Set("Content-Transfer-Encoding", "binary")

		var buff [1024 * 1024]byte
		for {
			n, err := f.Read(buff[:])
			if err != nil {
				common.LogAdd(common.MessError, "HandleGetFiles: error reading file")
				return
			}
			if n == 0 {
				common.LogAdd(common.MessError, "HandleGetFiles: file is closed")
				return
			}
				_, err = w.Write(buff[:n])
			if err != nil {
				common.LogAdd(common.MessError, "HandleGetFiles: error sending file")
				return
			}
		}
	}

	files, err := ioutil.ReadDir("files/" + client.Profile.Email + fileName)
	if err != nil {
		common.LogAdd(common.MessError, "HandleGetFiles: "+err.Error())
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}

	fileList := make([]FileDescription, 0)

	for _, f := range files {
		fileList = append(fileList, FileDescription{
			Name:  f.Name(),
			IsDir: f.IsDir(),
			Mode:  f.Mode(),
			Time:  f.ModTime(),
			Size:  f.Size(),
		})
	}

	b, err := json.Marshal(fileList)
	if err != nil {
		common.LogAdd(common.MessError, "HandleGetFiles: "+err.Error())
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
	err := curWsClient.ws.SetWriteDeadline(time.Now().Add(time.Second))
	if err != nil {
		curWsClient.mutex.Unlock()
		return err
	}
	err = curWsClient.ws.WriteJSON(v)
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

			common.LogAdd(common.MessFull, "chat "+client.Pid+" -> "+chat.Pid+": "+common.DecodeB64(chat.Text))

			destinationPid := chat.Pid
			chat.Pid = client.Pid
			b, err := json.Marshal(chat)
			if err != nil {
				continue
			}
			m.Data = string(b)

			list := GetWsClient(destinationPid)
			for i := 0; i < len(list); i++ {
				list[i].SendJSON(m)
			}
		}
	}

	common.LogAdd(common.MessInfo, "web ws disconnected")
	curWsClient.removeWsClient()
}
