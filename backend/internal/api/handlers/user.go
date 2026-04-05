package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jeremy-misola/kubesandbox/backend/internal/api/middleware"
)

// UserResponse represents the user information response
type UserResponse struct {
	ID     string   `json:"id"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Groups []string `json:"groups"`
}

// GetCurrentUser returns the current authenticated user
func GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	response := UserResponse{
		ID:     user.Email,
		Email:  user.Email,
		Name:   user.Name,
		Groups: user.Groups,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
