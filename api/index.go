// api/index.go
package handler

import (
	"net/http"
	"sync"

	"absensi/app"
)

var once sync.Once
var srv *app.Server

func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		var err error
		srv, err = app.NewFromEnv()
		if err != nil {
			panic(err)
		}
	})
	srv.Handler.ServeHTTP(w, r)
}
