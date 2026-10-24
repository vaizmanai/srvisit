package api

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/client"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
	"github.com/vaizmanai/srvisit/internal/pkg/profile"
	"io"
	"net/http"
	"os"
)

// HandleGetClientsList getting full online clients list
func HandleGetClientsList(w http.ResponseWriter, r *http.Request) {
	list := make([]clientResponse, 0)
	for _, c := range client.GetAllClientsList() {
		list = append(list, publicClient(c))
	}
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(list)
	if err != nil {
		log.Errorf("HandleGetClientsList: %s", err.Error())
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(b)
}

// HandleGetProfileList getting full profiles list
func HandleGetProfileList(w http.ResponseWriter, r *http.Request) {
	list := make([]profileResponse, 0)
	for _, p := range profile.GetProfileList() {
		list = append(list, publicProfile(p))
	}
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(list)
	if err != nil {
		log.Errorf("HandleGetProfileList: %s", err.Error())
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(b)
}

// HandleGetLog getting content of log file
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

// HandleDelLog clear log file
func HandleDelLog(w http.ResponseWriter, r *http.Request) {
	log.Infof("WEB Запрос очистки log")
	common.ReOpenLogFile()
}
