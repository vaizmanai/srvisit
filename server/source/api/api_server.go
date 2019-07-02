package api

import (
	"../common"
	"../component/client"
	"../component/profile"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
)

//Getting full online clients list
func HandleGetClientsList(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(client.GetAllClientsList())
	if err != nil {
		common.LogAdd(common.MessError, "HandleGetClientsList: "+err.Error())
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

//Getting full profiles list
func HandleGetProfileList(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(profile.GetProfileList())
	if err != nil {
		common.LogAdd(common.MessError, "HandleGetProfileList: "+err.Error())
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}

	w.Write(b)
}

//Getting content of log file
func HandleGetLog(w http.ResponseWriter, r *http.Request) {
	common.LogAdd(common.MessInfo, "WEB Запрос log")
	file, err := os.Open(common.LogFilename)
	if err != nil {
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	log, err := ioutil.ReadAll(file)
	if err != nil {
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}

	w.Write(log)
}

//Clear log file
func HandleDelLog(w http.ResponseWriter, r *http.Request) {
	common.LogAdd(common.MessInfo, "WEB Запрос очистки log")
	common.ClearLog()
}
