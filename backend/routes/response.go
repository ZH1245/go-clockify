package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/ZH1245/go-clockify/backend/controllers"
	"gorm.io/gorm"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// writeError maps controller errors to HTTP status codes, so controllers never need to know about HTTP.
func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	case errors.Is(err, controllers.ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, controllers.ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	default:
		log.Printf("internal error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

func pathID(r *http.Request) (uint, error) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("%w: id must be a positive number", controllers.ErrInvalidInput)
	}
	return uint(id), nil
}

func readJSON(r *http.Request, target any) error {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		return fmt.Errorf("%w: invalid JSON body: %v", controllers.ErrInvalidInput, err)
	}
	return nil
}
