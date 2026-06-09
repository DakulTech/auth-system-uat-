package routes

import (
	"net/http"

	"auth/internal/controllers"
	"auth/internal/services"
)

// RegisterRoutes wires the /register endpoint
func RegisterRoutes(mux *http.ServeMux, authService *services.AuthService) {
	registerController := &controllers.RegisterController{AuthService: authService}
	mux.HandleFunc("/register", registerController.RegisterHandler)
}
