// api/[...path].go
package handler

import (
	"absensi/app"
	"net/http"
	"strings"
	"sync"
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

	h := srv.Handler
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
		http.StripPrefix("/api", h).ServeHTTP(w, r)
		return
	}
	h.ServeHTTP(w, r)
}
