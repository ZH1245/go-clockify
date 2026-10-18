package routes

import (
	"net/http"

	"github.com/ZH1245/go-clockify/backend/controllers"
	"gorm.io/gorm"
)

func RegisterTaskItemRoutes(mux *http.ServeMux, db *gorm.DB) {
	// Auto entry: start a timer now; it runs until POST /items/{id}/stop.
	mux.HandleFunc("POST /tasks/{id}/start", func(w http.ResponseWriter, r *http.Request) {
		taskID, err := pathID(r)
		if err != nil {
			writeError(w, err)
			return
		}
		item, err := controllers.StartTimer(db, taskID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	})

	// Manual entry: {"start_at": "...", "manual_hours": 1.5}
	mux.HandleFunc("POST /tasks/{id}/items", func(w http.ResponseWriter, r *http.Request) {
		taskID, err := pathID(r)
		if err != nil {
			writeError(w, err)
			return
		}
		var input controllers.ManualEntryInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		item, err := controllers.AddManualEntry(db, taskID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	})

	mux.HandleFunc("POST /items/{id}/stop", func(w http.ResponseWriter, r *http.Request) {
		itemID, err := pathID(r)
		if err != nil {
			writeError(w, err)
			return
		}
		item, err := controllers.StopTimer(db, itemID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("PATCH /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		itemID, err := pathID(r)
		if err != nil {
			writeError(w, err)
			return
		}
		var input controllers.UpdateTaskItemInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		item, err := controllers.UpdateTaskItem(db, itemID, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("DELETE /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		itemID, err := pathID(r)
		if err != nil {
			writeError(w, err)
			return
		}
		if err := controllers.DeleteTaskItem(db, itemID); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
