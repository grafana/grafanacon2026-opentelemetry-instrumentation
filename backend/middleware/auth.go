package middleware

import (
	"context"
	"database/sql"
	"net/http"

	dbpkg "github.com/workshop/tapas-backend/db"
)

type contextKey string

const UserKey contextKey = "user"

// LoadUser reads the user-id header and attaches the user to the request context.
// If the header is absent or the user is not found, the request proceeds without a user.
func LoadUser(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := r.Header.Get("user-id")
			if userID != "" {
				var u dbpkg.User
				err := db.QueryRowContext(r.Context(),
					`SELECT id, username, is_admin, created_at FROM users WHERE id = $1`,
					userID,
				).Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt)
				if err == nil {
					ctx := context.WithValue(r.Context(), UserKey, &u)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetUser returns the user from the request context (may be nil).
func GetUser(r *http.Request) *dbpkg.User {
	u, _ := r.Context().Value(UserKey).(*dbpkg.User)
	return u
}

// RequireUser returns 401 if no user is in context.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetUser(r) == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"authentication required"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin returns 403 if the user is not an admin.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := GetUser(r)
		if u == nil || !u.IsAdmin {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"admin access required"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
