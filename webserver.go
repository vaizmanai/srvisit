package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"time"
)

func httpServer() {
	myRouter := mux.NewRouter().StrictSlash(false)

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
	apiRouter.HandleFunc("/auth", handleAuth).Methods("POST")
	apiRouter.Handle("/test", checkAuth(handleTest)).Methods("GET")
	apiRouter.Use(handleCORS)

	//-----------------------

	go func() {
		err := http.ListenAndServe(":"+options.HttpServerPort, myRouter)
		if err != nil {
			logAdd(MESS_ERROR, "webServer не смог занять порт: "+fmt.Sprint(err))
		}
	}()

	err := http.ListenAndServeTLS(":"+options.HttpsServerPort, options.HttpsCertPath, options.HttpsKeyPath, myRouter)
	if err != nil {
		logAdd(MESS_ERROR, "webServer не смог занять порт: "+fmt.Sprint(err))
	}

}

func checkAuth(f func(w http.ResponseWriter, r *http.Request, client *Client)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie_pid, err := r.Cookie("abc")
		if err != nil {
			http.Error(w, "unknown user", http.StatusUnauthorized)
			return
		}

		cookie_token, err := r.Cookie("abc")
		if err != nil {
			http.Error(w, "unknown user", http.StatusUnauthorized)
			return
		}

		list := clients[cleanPid(cookie_pid.Value)]
		for _, c := range list {
			if c.Token == cookie_token.Value {
				cookie_pid.Expires = time.Now().Add(WEB_TIMEOUT_HOUR * time.Hour)
				cookie_token.Expires = time.Now().Add(WEB_TIMEOUT_HOUR * time.Hour)
				http.SetCookie(w, cookie_pid)
				http.SetCookie(w, cookie_token)
				f(w, r, c)
				return
			}
		}

		http.Error(w, "unknown user", http.StatusUnauthorized)
	})
}

func handleCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logAdd(MESS_FULL, "get req: "+r.RequestURI)

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "6400")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, X-Requested-With, X-HTTP-Method-Override, Content-Type, Accept")

		if r.Method == "OPTIONS" {
			w.Write([]byte("ok"))
			return
		}

		h.ServeHTTP(w, r)
	})
}

func handleTest(w http.ResponseWriter, r *http.Request, client *Client) {
	b, _ := json.Marshal(client)
	w.Write(b)
}

func handleAuth(w http.ResponseWriter, r *http.Request) {
	pid := string(r.FormValue("abc"))
	token := string(r.FormValue("cba"))

	logAdd(MESS_INFO, "trying to auth app "+pid)

	list := clients[cleanPid(pid)]
	for _, c := range list {
		if c.Token == token {
			cookie_pid := http.Cookie{Name: "abc", Value: pid, Expires: time.Now().Add(WEB_TIMEOUT_HOUR * time.Hour)}
			cookie_token := http.Cookie{Name: "cba", Value: token, Expires: time.Now().Add(WEB_TIMEOUT_HOUR * time.Hour)}
			http.SetCookie(w, &cookie_pid)
			http.SetCookie(w, &cookie_token)
			return
		}
	}
	http.Error(w, "Launch by reVisit, please!", http.StatusUnauthorized)
}
