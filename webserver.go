package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
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
	apiRouter.Handle("/test", handleAuth(http.HandlerFunc(handleTest))).Methods("GET")
	apiRouter.Use(handleCORS)

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

func exampleMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logAdd(MESS_FULL, "check auth")

		// Our middleware logic goes here...
		next.ServeHTTP(w, r)
	})
}

func handleAuth(f func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logAdd(MESS_FULL, "check auth")

		pid := string(r.FormValue("abc"))
		token := string(r.FormValue("cba"))

		fmt.Println(pid)
		fmt.Println(token)

		list := clients[cleanPid(pid)]
		for _, c := range list {
			if c.token == token {
				f(w, r)

				return
			}
		}

		http.Error(w, "req auth", http.StatusUnauthorized)
		return
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

func handleTest(w http.ResponseWriter, r *http.Request) {
	//w.Write([]byte("test"))
	//w.WriteHeader(http.StatusInternalServerError)
	http.Error(w, "123", http.StatusInternalServerError)
}
