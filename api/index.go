// api/[...path].go
package handler

import (
	"net/http"
	"sync"

	"absensi/app"
)

var once sync.Once
var h http.Handler

func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		srv, err := app.NewFromEnv()
		if err != nil {
			panic(err)
		}
		// request ke /api/... jadi /...
		h = http.StripPrefix("/api", srv.Handler)
	})
	h.ServeHTTP(w, r)
}
