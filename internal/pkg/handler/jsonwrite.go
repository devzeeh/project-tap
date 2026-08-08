package jsonwrite

import (
	"encoding/json"
	"net/http"
)

// Create a standard API response struct
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Data    any    `json:"data,omitempty"` // Optional field for additional data
}

// Login specific response — returns user data after login
type LoginResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	ID          string `json:"id,omitempty"` // Optional: include user ID in response
	Username    string `json:"username"`     // Optional: include user ID in response
	RedirectURL string `json:"redirect_url,omitempty"`
}

// Auth Handler (POST) - Converted to JSON API
func WriteJSON(w http.ResponseWriter, status int, resp any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}
