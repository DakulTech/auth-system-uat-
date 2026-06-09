package routes

import (
	"net/http"

	"auth/internal/controllers"
	"database/sql"
)

// VerifyRoutes wires the /verify endpoint
func VerifyRoutes(mux *http.ServeMux, db *sql.DB) {
	verifyController := &controllers.VerifyController{DB: db}
	mux.HandleFunc("/verify", verifyController.VerifyHandler)
}
