package routes

import (
	"net/http"

	"github.com/ZH1245/go-clockify/backend/controllers"
	"gorm.io/gorm"
)

func RegisterProjectRoutes(mux *http.ServeMux, db *gorm.DB) {
	mux.HandleFunc("GET /projects", func(w http.ResponseWriter, r *http.Request) {
		projects, err := controllers.GetProjects(r.Context(), db)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, projects)
	})

	mux.HandleFunc("GET /projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeError(w, err)
			return
		}
		project, err := controllers.GetProjectByID(r.Context(), db, id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, project)
	})

	mux.HandleFunc("POST /projects", func(w http.ResponseWriter, r *http.Request) {
		var input controllers.CreateProjectInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		project, err := controllers.CreateProject(r.Context(), db, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, project)
	})

	mux.HandleFunc("PATCH /projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeError(w, err)
			return
		}
		var input controllers.UpdateProjectInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		project, err := controllers.UpdateProject(r.Context(), db, id, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, project)
	})

	mux.HandleFunc("DELETE /projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeError(w, err)
			return
		}
		if err := controllers.DeleteProject(r.Context(), db, id); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
