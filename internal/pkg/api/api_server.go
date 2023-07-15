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
	b, err := json.Marshal(client.GetAllClientsList())
	if err != nil {
		log.Errorf("HandleGetClientsList: %s", err.Error())
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(b)
}

// HandleGetProfileList getting full profiles list
func HandleGetProfileList(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(profile.GetProfileList())
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
	common.CloseLogFile()
}
