package api

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/client"
	"net/http"
)

// HandleGetClient getting authorized client info
func HandleGetClient(w http.ResponseWriter, r *http.Request, client *client.Client) {
	b, err := json.Marshal(client)
	if err != nil {
		log.Errorf("handleGetClient: %s", err.Error())
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(b)
}
