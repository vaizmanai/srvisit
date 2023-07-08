package api

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"srvisit/internal/pkg/client"
	"srvisit/internal/pkg/common"
	"srvisit/internal/pkg/profile"
)

// Getting full online clients list
func HandleGetClientsList(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(client.GetAllClientsList())
	if err != nil {
		log.Errorf("HandleGetClientsList: %s", err.Error())
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(b)
}

// Getting full profiles list
func HandleGetProfileList(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(profile.GetProfileList())
	if err != nil {
		log.Errorf("HandleGetProfileList: %s", err.Error())
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(b)
}

// Getting content of log file
func HandleGetLog(w http.ResponseWriter, r *http.Request) {
	log.Infof("WEB Запрос log")
	file, err := os.Open(common.LogName)
	if err != nil {
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(b)
}

// Clear log file
func HandleDelLog(w http.ResponseWriter, r *http.Request) {
	log.Infof("WEB Запрос очистки log")
	common.CloseLogFile()
}
