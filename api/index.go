// api/index.go
package handler

import (
	"absensi/app"
	"log"
	"net/http"
	"sync"
)

var once sync.Once
var srv *app.Server

func Handler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[panic] %v", rec)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	}()

	once.Do(func() {
		var err error
		srv, err = app.NewFromEnv()
		if err != nil {
			// biar lognya kebaca di vercel logs
			log.Printf("init error: %v", err)
			panic(err)
		}
	})

	srv.Handler.ServeHTTP(w, r)
}
