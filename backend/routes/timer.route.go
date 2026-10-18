package routes

import (
	"net/http"

	"github.com/ZH1245/go-clockify/backend/controllers"
	"gorm.io/gorm"
)

func RegisterTimerRoutes(mux *http.ServeMux, db *gorm.DB) {
	// Heartbeat: the frontend calls this every few seconds while a tab is open. There is no
	// body and no id - it just refreshes every running timer. A timer that stops being
	// pinged is closed by the sweeper in internal/sweeper.
	mux.HandleFunc("POST /timers/ping", func(w http.ResponseWriter, r *http.Request) {
		running, err := controllers.PingTimers(r.Context(), db)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]int64{"running": running})
	})
}
