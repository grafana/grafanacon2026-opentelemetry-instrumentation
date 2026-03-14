package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	dbpkg "github.com/workshop/tapas-backend/db"
	"github.com/workshop/tapas-backend/handlers"
	"github.com/workshop/tapas-backend/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
)

func main() {
	// Fallback logger (console) — replaced by OTel bridge if telemetry setup succeeds.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	ctx := context.Background()
	shutdown, err := setupTelemetry(ctx)
	if err != nil {
		slog.Error("failed to setup telemetry", "error", err)
	} else {
		defer shutdown(ctx)
	}

	db, err := dbpkg.Connect()
	if err != nil {
		log.Fatalf("cannot connect to database: %v", err)
	}
	defer db.Close()

	r := mux.NewRouter()
	r.Use(otelmux.Middleware("backend",
		otelmux.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/api/health"
		}),
	))
	r.Use(middleware.Logging)
	r.Use(middleware.LoadUser(db))

	api := r.PathPrefix("/api").Subrouter()

	// Health
	api.Handle("/health", handlers.Health(db)).Methods(http.MethodGet)

	// Restaurants
	api.Handle("/restaurants", handlers.ListRestaurants(db)).Methods(http.MethodGet)
	api.Handle("/restaurants/{id}", handlers.GetRestaurant(db)).Methods(http.MethodGet)
	api.Handle("/restaurants",
		middleware.RequireAdmin(handlers.CreateRestaurant(db))).Methods(http.MethodPost)
	api.Handle("/restaurants/{id}",
		middleware.RequireAdmin(handlers.UpdateRestaurant(db))).Methods(http.MethodPut)
	api.Handle("/restaurants/{id}",
		middleware.RequireAdmin(handlers.DeleteRestaurant(db))).Methods(http.MethodDelete)

	// Photos
	api.Handle("/restaurants/{id}/photos",
		middleware.RequireAdmin(handlers.UploadPhoto(db))).Methods(http.MethodPost)
	api.Handle("/restaurants/{id}/photos/{photo_id}",
		handlers.GetPhoto(db)).Methods(http.MethodGet)
	api.Handle("/restaurants/{id}/photos/{photo_id}",
		middleware.RequireAdmin(handlers.DeletePhoto(db))).Methods(http.MethodDelete)

	// Ratings
	api.Handle("/restaurants/{id}/ratings",
		middleware.RequireUser(handlers.SubmitRating(db))).Methods(http.MethodPost)
	api.Handle("/restaurants/{id}/ratings",
		handlers.ListRatings(db)).Methods(http.MethodGet)

	// Users
	api.Handle("/users",
		middleware.RequireAdmin(handlers.ListUsers(db))).Methods(http.MethodGet)
	api.Handle("/users",
		handlers.CreateUser(db)).Methods(http.MethodPost)
	api.Handle("/users/by-username/{username}",
		handlers.GetUserByUsername(db)).Methods(http.MethodGet)
	api.Handle("/users/me/favorites",
		middleware.RequireUser(handlers.GetFavorites(db))).Methods(http.MethodGet)

	handler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "user-id"},
	}).Handler(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprintf(":%s", port)
	slog.Info("backend listening", "addr", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
