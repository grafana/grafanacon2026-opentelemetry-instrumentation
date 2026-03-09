package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	dbpkg "github.com/workshop/tapas-backend/db"
	"github.com/workshop/tapas-backend/middleware"
)

func ListUsers(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.QueryContext(r.Context(),
			`SELECT id, username, is_admin, created_at FROM users ORDER BY username`)
		if err != nil {
			Err(w, http.StatusInternalServerError, "database error")
			return
		}
		defer rows.Close()

		var users []dbpkg.User
		for rows.Next() {
			var u dbpkg.User
			if err := rows.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt); err != nil {
				continue
			}
			users = append(users, u)
		}
		if users == nil {
			users = []dbpkg.User{}
		}
		JSON(w, http.StatusOK, map[string]any{"users": users})
	}
}

func GetUserByUsername(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := mux.Vars(r)["username"]
		var u dbpkg.User
		err := db.QueryRowContext(r.Context(),
			`SELECT id, username, is_admin, created_at FROM users WHERE username = $1`,
			username).Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt)
		if err == sql.ErrNoRows {
			Err(w, http.StatusNotFound, "user not found")
			return
		}
		if err != nil {
			Err(w, http.StatusInternalServerError, "database error")
			return
		}
		JSON(w, http.StatusOK, u)
	}
}

func CreateUser(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" {
			Err(w, http.StatusBadRequest, "username is required")
			return
		}
		var u dbpkg.User
		err := db.QueryRowContext(r.Context(),
			`INSERT INTO users (id, username, is_admin) VALUES ($1, $2, false)
			 RETURNING id, username, is_admin, created_at`,
			newID(), body.Username).Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt)
		if err != nil {
			Err(w, http.StatusConflict, "username already taken")
			return
		}
		JSON(w, http.StatusCreated, u)
	}
}

func GetFavorites(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller := middleware.GetUser(r)

		query := `
			SELECT ` + listSelect + `
			FROM restaurants r
			WHERE EXISTS (
				SELECT 1 FROM ratings rt
				WHERE rt.restaurant_id = r.id
				  AND rt.user_id = $1
				  AND rt.rating >= 4
			)
			ORDER BY r.avg_rating DESC`

		rows, err := db.QueryContext(r.Context(), query, caller.ID)
		if err != nil {
			Err(w, http.StatusInternalServerError, "database error")
			return
		}
		defer rows.Close()

		var restaurants []*dbpkg.Restaurant
		for rows.Next() {
			rest, err := dbpkg.ScanRestaurant(rows, false)
			if err != nil {
				Err(w, http.StatusInternalServerError, "scan error")
				return
			}
			restaurants = append(restaurants, rest)
		}
		if restaurants == nil {
			restaurants = []*dbpkg.Restaurant{}
		}
		JSON(w, http.StatusOK, map[string]any{
			"restaurants": restaurants,
			"total":       len(restaurants),
		})
	}
}
