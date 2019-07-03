package api

import (
	"../common"
	"../component/client"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
	"time"
)

const (
	pausePingSec    = 5
	readDeadlineSec = 10

	wsPing    = 0
	wsMessage = 1
)

type websocketMessage struct {
	Type int
	Data string
}

//Getting authorized client info
func HandleGetClient(w http.ResponseWriter, r *http.Request, client *client.Client) {
	b, err := json.Marshal(client)
	if err != nil {
		common.LogAdd(common.MessError, "handleGetClient: "+err.Error())
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}
	w.Write(b)
}

func HandleChatWS(ws *websocket.Conn, client *client.Client) {
	common.LogAdd(common.MessInfo, "web ws connected")
	if ws == nil {
		common.LogAdd(common.MessError, "web ws is null")
		return
	}

	mutex := sync.Mutex{}

	go func() {
		m := websocketMessage{
			Type: wsPing,
			Data: "",
		}

		for {
			time.Sleep(time.Second * pausePingSec)
			if ws == nil {
				common.LogAdd(common.MessError, "error writing json: closed connection")
				return
			}

			mutex.Lock()
			err := ws.WriteJSON(m)
			mutex.Unlock()

			if err != nil {
				common.LogAdd(common.MessError, "error writing json: "+fmt.Sprint(err))
				return
			}
		}
	}()

	for {
		m := websocketMessage{}

		err := ws.SetReadDeadline(time.Now().Add(time.Second * readDeadlineSec))
		if err != nil {
			common.LogAdd(common.MessError, "error reading json: "+fmt.Sprint(err))
			//todo remove from list connection
			break
		}

		err = ws.ReadJSON(&m)
		if err != nil {
			common.LogAdd(common.MessError, "error reading json: "+fmt.Sprint(err))
			//todo remove from list connection
			break
		}

		common.LogAdd(common.MessDetail, fmt.Sprint(m))

		if m.Type == wsPing {

		} else if m.Type == wsMessage {

		}
	}

	//todo remove from list connection
	common.LogAdd(common.MessInfo, "web ws disconnected")
}
