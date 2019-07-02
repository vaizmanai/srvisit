package api

import (
	"../common"
	"../component/client"
	"encoding/json"
	"net/http"
)

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
