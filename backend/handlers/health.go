package handlers

import (
	"net/http"

	dbpkg "github.com/workshop/tapas-backend/db"
)

func Health(db *dbpkg.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		if err := db.PingContext(r.Context()); err != nil {
			dbStatus = "error"
		}
		JSON(w, http.StatusOK, map[string]string{"status": "ok", "db": dbStatus})
	}
}
