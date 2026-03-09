package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/workshop/tapas-backend/middleware"
)

type ratingInput struct {
	Rating int `json:"rating"`
}

func SubmitRating(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := middleware.GetUser(r)
		slug := mux.Vars(r)["id"]

		var restaurantID string
		if err := db.QueryRowContext(r.Context(),
			`SELECT id FROM restaurants WHERE slug = $1`, slug,
		).Scan(&restaurantID); err == sql.ErrNoRows {
			Err(w, http.StatusNotFound, "restaurant not found")
			return
		} else if err != nil {
			Err(w, http.StatusInternalServerError, "database error")
			return
		}

		var inp ratingInput
		if err := json.NewDecoder(r.Body).Decode(&inp); err != nil {
			Err(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if inp.Rating < 1 || inp.Rating > 5 {
			Err(w, http.StatusBadRequest, "rating must be between 1 and 5")
			return
		}

		// Upsert rating atomically; xmax=0 means the row was just inserted (not updated).
		var isNew bool
		err := db.QueryRowContext(r.Context(),
			`INSERT INTO ratings (id, user_id, restaurant_id, rating)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (user_id, restaurant_id) DO UPDATE SET rating = EXCLUDED.rating
			 RETURNING (xmax = 0)`,
			newID(), u.ID, restaurantID, inp.Rating,
		).Scan(&isNew)
		if err != nil {
			Err(w, http.StatusInternalServerError, "could not save rating")
			return
		}

		// Recompute avg_rating
		var newAvg float64
		_ = db.QueryRowContext(r.Context(),
			`UPDATE restaurants SET avg_rating = (
				SELECT ROUND(AVG(rating)::NUMERIC, 2) FROM ratings WHERE restaurant_id = $1
			) WHERE id = $1 RETURNING avg_rating`,
			restaurantID,
		).Scan(&newAvg)

		status := http.StatusOK
		if isNew {
			status = http.StatusCreated
		}
		JSON(w, status, map[string]any{
			"restaurant_id":  restaurantID,
			"rating":         inp.Rating,
			"new_avg_rating": newAvg,
		})
	}
}

func ListRatings(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := mux.Vars(r)["id"]

		var restaurantID string
		if err := db.QueryRowContext(r.Context(),
			`SELECT id FROM restaurants WHERE slug = $1`, slug,
		).Scan(&restaurantID); err == sql.ErrNoRows {
			Err(w, http.StatusNotFound, "restaurant not found")
			return
		} else if err != nil {
			Err(w, http.StatusInternalServerError, "database error")
			return
		}

		rows, err := db.QueryContext(r.Context(), `
			SELECT rt.user_id, u.username, rt.rating
			FROM ratings rt
			JOIN users u ON u.id = rt.user_id
			WHERE rt.restaurant_id = $1
			ORDER BY rt.created_at DESC`,
			restaurantID)
		if err != nil {
			Err(w, http.StatusInternalServerError, "database error")
			return
		}
		defer rows.Close()

		type ratingEntry struct {
			UserID   string `json:"user_id"`
			Username string `json:"username"`
			Rating   int    `json:"rating"`
		}
		var list []ratingEntry
		for rows.Next() {
			var e ratingEntry
			if err := rows.Scan(&e.UserID, &e.Username, &e.Rating); err != nil {
				continue
			}
			list = append(list, e)
		}
		if list == nil {
			list = []ratingEntry{}
		}

		// Use the pre-computed, consistently-rounded avg_rating from the restaurants table.
		var avg float64
		_ = db.QueryRowContext(r.Context(),
			`SELECT avg_rating FROM restaurants WHERE id = $1`, restaurantID,
		).Scan(&avg)

		JSON(w, http.StatusOK, map[string]any{
			"ratings":    list,
			"avg_rating": avg,
			"count":      len(list),
		})
	}
}
