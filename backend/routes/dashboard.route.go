package routes

import (
	"context"
	"net/http"
	"time"

	"github.com/ZH1245/go-clockify/backend/controllers"
	"gorm.io/gorm"
)

func RegisterDashboardRoutes(mux *http.ServeMux, db *gorm.DB) {
	mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
		// Give the whole dashboard a 5s budget. r.Context() is already cancelled when the
		// client disconnects; WithTimeout derives a new context that also expires on its own,
		// and cancel() releases its resources as soon as the handler returns.
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		dashboard, err := controllers.GetDashboard(ctx, db)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, dashboard)
	})
}
