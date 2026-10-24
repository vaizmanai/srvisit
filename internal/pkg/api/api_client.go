package api

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/client"
	"github.com/vaizmanai/srvisit/internal/pkg/profile"
	"net/http"
)

type profileResponse struct {
	Email string
	Capt  string
	Tel   string
	Logo  string
}

type clientResponse struct {
	Serial  string
	Pid     string
	Version string
	Profile *profileResponse
}

func publicProfile(p *profile.Profile) profileResponse {
	p.Lock()
	defer p.Unlock()
	return profileResponse{p.Email, p.Capt, p.Tel, p.Logo}
}

func publicClient(c *client.Client) clientResponse {
	snapshot := c.Snapshot()
	result := clientResponse{Serial: snapshot.Serial, Pid: snapshot.Pid, Version: snapshot.Version}
	if snapshot.Profile != nil {
		p := publicProfile(snapshot.Profile)
		result.Profile = &p
	}
	return result
}

// HandleGetClient getting authorized client info
func HandleGetClient(w http.ResponseWriter, r *http.Request, client *client.Client) {
	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(publicClient(client))
	if err != nil {
		log.Errorf("handleGetClient: %s", err.Error())
		http.Error(w, "couldn't service this request", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(b)
}
