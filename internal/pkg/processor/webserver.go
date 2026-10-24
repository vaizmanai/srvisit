package processor

import (
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/api"
	"github.com/vaizmanai/srvisit/internal/pkg/client"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func HttpServer() {
	config := common.GetOptions()
	myRouter := newRouter()
	makeServer := func(port string) *http.Server {
		return &http.Server{
			Addr: ":" + port, Handler: myRouter,
			ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second,
			WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
			MaxHeaderBytes: 32 << 10,
		}
	}
	if config.HttpServerEnabled {
		go func() {
			if err := makeServer(config.HttpServerPort).ListenAndServe(); err != nil {
				log.Fatalf("webServer: %s", err)
			}
		}()
	}
	if config.HttpsServerEnabled {
		go func() {
			if err := makeServer(config.HttpsServerPort).ListenAndServeTLS(config.HttpsCertPath, config.HttpsKeyPath); err != nil {
				log.Fatalf("webServer TLS: %s", err)
			}
		}()
	}
}

func newRouter() http.Handler {
	myRouter := mux.NewRouter().StrictSlash(true)

	//-----------------------

	myRouter.Handle("/admin", http.RedirectHandler("/admin/welcome", 301))
	myRouter.HandleFunc("/admin/welcome", handleWelcome)
	myRouter.HandleFunc("/admin/resources", handleResources)
	myRouter.HandleFunc("/admin/statistics", handleStatistics)
	myRouter.HandleFunc("/admin/options", handleOptions)
	myRouter.HandleFunc("/admin/logs", handleLogs)

	myRouter.Handle("/", http.RedirectHandler("/profile/welcome", 301))
	myRouter.Handle("/profile", http.RedirectHandler("/profile/welcome", 301))
	myRouter.HandleFunc("/profile/welcome", handleProfileWelcome)
	myRouter.HandleFunc("/profile/my", handleProfileMy)

	myRouter.PathPrefix("/resource/").HandlerFunc(handleResource)
	myRouter.HandleFunc("/api", handleAPI)

	//-----------------------

	apiRouter := myRouter.PathPrefix("/v2/api").Subrouter()
	apiRouter.Use(handleCORS)
	apiRouter.PathPrefix("/").Methods(http.MethodOptions).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	apiRouter.HandleFunc("/auth", handleAuth).Methods(http.MethodGet, http.MethodPost)
	apiRouter.Handle("/client", checkAuth(api.HandleGetClient)).Methods(http.MethodGet)

	apiAdmin := apiRouter.PathPrefix("/admin").Subrouter()
	apiAdmin.Handle("/clients", checkAdmin(api.HandleGetClientsList)).Methods(http.MethodGet)
	apiAdmin.Handle("/profiles", checkAdmin(api.HandleGetProfileList)).Methods(http.MethodGet)
	apiAdmin.Handle("/log", checkAdmin(api.HandleGetLog)).Methods(http.MethodGet)
	apiAdmin.Handle("/log", checkAdmin(api.HandleDelLog)).Methods(http.MethodDelete)

	//-----------------------

	return myRouter
}

func checkAuth(f func(w http.ResponseWriter, r *http.Request, client *client.Client)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pidCookie, err := r.Cookie("abc")
		if err != nil {
			http.Error(w, "unknown user", http.StatusUnauthorized)
			return
		}

		tokenCookie, err := r.Cookie("cba")
		if err != nil {
			http.Error(w, "unknown user", http.StatusUnauthorized)
			return
		}

		list := client.GetClientsList(pidCookie.Value)
		for _, c := range list {
			if c.MatchesToken(tokenCookie.Value) {
				setSessionCookies(w, r, pidCookie.Value, tokenCookie.Value)
				f(w, r, c)
				return
			}
		}

		http.Error(w, "unknown user", http.StatusUnauthorized)
	}
}

func checkAdmin(f func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if checkAdminAuth(w, r) {
			f(w, r)
		}
	}
}

func handleCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Max-Age", "6400")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, X-Requested-With, X-HTTP-Method-Override, Content-Type, Accept")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func handleAuth(w http.ResponseWriter, r *http.Request) {
	pid := r.FormValue("abc")
	token := r.FormValue("cba")
	destination := r.FormValue("destination")
	if destination == "" {
		destination = "/"
	}
	target, err := url.Parse(destination)
	if err != nil || target.IsAbs() || target.Host != "" || !strings.HasPrefix(destination, "/") || strings.HasPrefix(destination, "//") || strings.ContainsAny(destination, "\\\r\n") {
		http.Error(w, "invalid destination", http.StatusBadRequest)
		return
	}

	log.Infof("trying to auth app %s", pid)

	list := client.GetClientsList(pid)
	for _, c := range list {
		state := c.Snapshot()
		if c.MatchesToken(token) && state.Conn != nil && *state.Conn != nil {
			clientIp, _, clientErr := net.SplitHostPort((*state.Conn).RemoteAddr().String())
			webIp, _, webErr := net.SplitHostPort(r.RemoteAddr)
			if clientErr != nil || webErr != nil || net.ParseIP(webIp) == nil || !net.ParseIP(webIp).Equal(net.ParseIP(clientIp)) {
				continue
			}

			setSessionCookies(w, r, pid, token)
			http.Redirect(w, r, destination, http.StatusSeeOther)
			return
		}
	}
	http.Error(w, "Launch by reVisit, please!", http.StatusUnauthorized)
}

func setSessionCookies(w http.ResponseWriter, r *http.Request, pid, token string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	for name, value := range map[string]string{"abc": pid, "cba": token} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: value, Path: "/", HttpOnly: true,
			Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode,
			Expires: time.Now().Add(common.WebSessionTimeoutHour * time.Hour),
		})
	}
}
