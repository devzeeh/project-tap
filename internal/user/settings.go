package user

import (
	"fmt"
	"net/http"
)

// SettingsView handles the display of the user's settings page
func (h *Handler) SettingsView(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Settings view is running...")

	username := r.PathValue("username")
	user, err := h.GetDashboardUser(username)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := struct {
		Username string
		User     DashboardUser
	}{
		Username: username,
		User:     user,
	}

	h.Tpl.ExecuteTemplate(w, "settings.html", data)
}
