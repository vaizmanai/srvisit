package processor

import (
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/srvisit/internal/pkg/api"
	"github.com/vaizmanai/srvisit/internal/pkg/client"
	"github.com/vaizmanai/srvisit/internal/pkg/common"
	"net"
	"net/http"
	"time"
)

func HttpServer() {
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

	myRouter.PathPrefix("/resource").HandlerFunc(handleResource)
	myRouter.HandleFunc("/api", handleAPI)

	//-----------------------

	apiRouter := myRouter.PathPrefix("/v2/api").Subrouter()
	apiRouter.Use(handleCORS)
	apiRouter.HandleFunc("/auth", handleAuth).Methods(http.MethodGet, http.MethodPost)
	apiRouter.Handle("/client", checkAuth(api.HandleGetClient)).Methods(http.MethodGet)

	apiAdmin := apiRouter.PathPrefix("/admin").Subrouter()
	apiAdmin.Handle("/clients", checkAdmin(api.HandleGetClientsList)).Methods(http.MethodGet)
	apiAdmin.Handle("/profiles", checkAdmin(api.HandleGetProfileList)).Methods(http.MethodGet)
	apiAdmin.Handle("/log", checkAdmin(api.HandleGetLog)).Methods(http.MethodGet)
	apiAdmin.Handle("/log", checkAdmin(api.HandleDelLog)).Methods(http.MethodDelete)

	//-----------------------

	if common.Options.HttpServerEnabled {
		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%s", common.Options.HttpServerPort), myRouter); err != nil {
				log.Fatalf("webServer не смог занять порт: %s", err.Error())
			}
		}()
	}

	if common.Options.HttpsServerEnabled {
		go func() {
			if err := http.ListenAndServeTLS(fmt.Sprintf(":%s", common.Options.HttpsServerPort), common.Options.HttpsCertPath, common.Options.HttpsKeyPath, myRouter); err != nil {
				log.Fatalf("webServer не смог занять порт: %s", err.Error())
			}
		}()
	}
}

func checkAuth(f func(w http.ResponseWriter, r *http.Request, client *client.Client)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pidCookie, err := r.Cookie("abc")
		if err != nil {
			http.Error(w, "unknown user", http.StatusUnauthorized)
			return
		}

		tokenCookie, err := r.Cookie("abc")
		if err != nil {
			http.Error(w, "unknown user", http.StatusUnauthorized)
			return
		}

		list := client.GetClientsList(pidCookie.Value)
		for _, c := range list {
			if c.Token == tokenCookie.Value {
				pidCookie.Expires = time.Now().Add(common.WebSessionTimeoutHour * time.Hour)
				tokenCookie.Expires = time.Now().Add(common.WebSessionTimeoutHour * time.Hour)
				http.SetCookie(w, pidCookie)
				http.SetCookie(w, tokenCookie)
				f(w, r, c)
				return
			}
		}

		http.Error(w, "unknown user", http.StatusUnauthorized)
	}
}

func checkAdmin(f func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if ok {
			if user == common.Options.AdminLogin && pass == common.Options.AdminPass {
				f(w, r)
				return
			}
		}

		time.Sleep(time.Second)
		log.Errorf("WWW Аутентификация провалилась %s", r.RemoteAddr)
		w.Header().Set("WWW-Authenticate", "Basic")
		http.Error(w, "auth req", http.StatusUnauthorized)
	}
}

func handleCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "6400")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, X-Requested-With, X-HTTP-Method-Override, Content-Type, Accept")

		if r.Method == http.MethodOptions {
			_, _ = w.Write([]byte("ok"))
			return
		}

		h.ServeHTTP(w, r)
	})
}

func handleAuth(w http.ResponseWriter, r *http.Request) {
	pid := r.FormValue("abc")
	token := r.FormValue("cba")
	destination := r.FormValue("destination")

	log.Infof("trying to auth app %s", pid)

	list := client.GetClientsList(pid)
	for _, c := range list {
		if c.Token == token {
			clientIp, _, _ := net.SplitHostPort((*c.Conn).RemoteAddr().String())
			webIp, _, _ := net.SplitHostPort(r.RemoteAddr)
			if webIp != clientIp {
				continue
			}

			cookiePid := http.Cookie{Name: "abc", Value: pid, Expires: time.Now().Add(common.WebSessionTimeoutHour * time.Hour)}
			cookieToken := http.Cookie{Name: "cba", Value: token, Expires: time.Now().Add(common.WebSessionTimeoutHour * time.Hour)}
			http.SetCookie(w, &cookiePid)
			http.SetCookie(w, &cookieToken)
			http.Redirect(w, r, destination, http.StatusTemporaryRedirect)
			return
		}
	}
	http.Error(w, "Launch by reVisit, please!", http.StatusUnauthorized)
}
