package routes

import (
	"net/http"

	"github.com/ZH1245/go-clockify/backend/controllers"
	"github.com/ZH1245/go-clockify/backend/models"
	"gorm.io/gorm"
)

// taskResponse is what the UI receives: the task with its items, plus totals computed from them.
type taskResponse struct {
	models.Task
	TotalHours float64 `json:"total_hours"`
	TimerCount int     `json:"timer_count"`
	Running    bool    `json:"running"`
}

func toTaskResponse(task models.Task) taskResponse {
	if task.Items == nil {
		task.Items = []models.TaskItem{}
	}
	return taskResponse{
		Task:       task,
		TotalHours: task.TotalHours(),
		TimerCount: len(task.Items),
		Running:    task.HasRunningItem(),
	}
}

func RegisterTaskRoutes(mux *http.ServeMux, db *gorm.DB) {
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {
		tasks, err := controllers.GetTasks(r.Context(), db)
		if err != nil {
			writeError(w, err)
			return
		}
		response := make([]taskResponse, len(tasks))
		for i, task := range tasks {
			response[i] = toTaskResponse(task)
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("GET /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeError(w, err)
			return
		}
		task, err := controllers.GetTaskByID(r.Context(), db, id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toTaskResponse(task))
	})

	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {
		var input controllers.CreateTaskInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		task, err := controllers.CreateTask(r.Context(), db, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, toTaskResponse(task))
	})

	mux.HandleFunc("PATCH /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeError(w, err)
			return
		}
		var input controllers.UpdateTaskInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		task, err := controllers.UpdateTask(r.Context(), db, id, input)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toTaskResponse(task))
	})

	// PUT /tasks/{id}/project — body {"project_id": 3} moves the task, {"project_id": null} clears it.
	mux.HandleFunc("PUT /tasks/{id}/project", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeError(w, err)
			return
		}
		var input controllers.SetTaskProjectInput
		if err := readJSON(r, &input); err != nil {
			writeError(w, err)
			return
		}
		task, err := controllers.SetTaskProject(r.Context(), db, id, input.ProjectID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toTaskResponse(task))
	})

	mux.HandleFunc("DELETE /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeError(w, err)
			return
		}
		if err := controllers.DeleteTask(r.Context(), db, id); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
