package backend_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	dbpkg "github.com/workshop/tapas-backend/db"
	"github.com/workshop/tapas-backend/handlers"
	"github.com/workshop/tapas-backend/middleware"
)

// ── Test setup ────────────────────────────────────────────────────────────────

var (
	testDB     *sql.DB
	testServer *httptest.Server
	adminID    = "a1f3e2d4"
	aliceID    = "b2c4d5e6"
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DB_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/tapas?sslmode=disable"
	}

	var err error
	testDB, err = sql.Open("postgres", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	if err := testDB.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "ping db: %v\n", err)
		os.Exit(1)
	}

	r := mux.NewRouter()
	r.Use(middleware.LoadUser(testDB))
	api := r.PathPrefix("/api").Subrouter()

	api.Handle("/health", handlers.Health(testDB)).Methods(http.MethodGet)
	api.Handle("/restaurants", handlers.ListRestaurants(testDB)).Methods(http.MethodGet)
	api.Handle("/restaurants/{id}", handlers.GetRestaurant(testDB)).Methods(http.MethodGet)
	api.Handle("/restaurants",
		middleware.RequireAdmin(handlers.CreateRestaurant(testDB))).Methods(http.MethodPost)
	api.Handle("/restaurants/{id}",
		middleware.RequireAdmin(handlers.UpdateRestaurant(testDB))).Methods(http.MethodPut)
	api.Handle("/restaurants/{id}",
		middleware.RequireAdmin(handlers.DeleteRestaurant(testDB))).Methods(http.MethodDelete)
	api.Handle("/restaurants/{id}/ratings",
		middleware.RequireUser(handlers.SubmitRating(testDB))).Methods(http.MethodPost)
	api.Handle("/restaurants/{id}/ratings",
		handlers.ListRatings(testDB)).Methods(http.MethodGet)
	api.Handle("/users",
		middleware.RequireAdmin(handlers.ListUsers(testDB))).Methods(http.MethodGet)
	api.Handle("/users/me/favorites",
		middleware.RequireUser(handlers.GetFavorites(testDB))).Methods(http.MethodGet)

	testServer = httptest.NewServer(r)
	defer testServer.Close()

	os.Exit(m.Run())
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func get(t *testing.T, path string, userID string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, testServer.URL+path, nil)
	if userID != "" {
		req.Header.Set("user-id", userID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func postJSON(t *testing.T, path string, body any, userID string) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, testServer.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		req.Header.Set("user-id", userID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func putJSON(t *testing.T, path string, body any, userID string) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, testServer.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		req.Header.Set("user-id", userID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", path, err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		resp.Body.Close()
		t.Fatalf("expected status %d, got %d", want, resp.StatusCode)
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestHealth(t *testing.T) {
	resp := get(t, "/api/health", "")
	assertStatus(t, resp, http.StatusOK)
	var body map[string]string
	decodeJSON(t, resp, &body)
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %q", body["status"])
	}
	if body["db"] != "ok" {
		t.Errorf("expected db ok, got %q", body["db"])
	}
}

func TestListRestaurants(t *testing.T) {
	resp := get(t, "/api/restaurants", "")
	assertStatus(t, resp, http.StatusOK)
	var body struct {
		Restaurants []dbpkg.Restaurant `json:"restaurants"`
		Total       int                `json:"total"`
	}
	decodeJSON(t, resp, &body)
	if body.Total == 0 {
		t.Error("expected at least one restaurant")
	}
	if len(body.Restaurants) != body.Total {
		t.Errorf("total mismatch: len=%d total=%d", len(body.Restaurants), body.Total)
	}
}

func TestListRestaurantsFilterByName(t *testing.T) {
	resp := get(t, "/api/restaurants?q=dragon", "")
	assertStatus(t, resp, http.StatusOK)
	var body struct {
		Restaurants []dbpkg.Restaurant `json:"restaurants"`
	}
	decodeJSON(t, resp, &body)
	for _, r := range body.Restaurants {
		name := r.Name
		if len(name) == 0 {
			t.Error("empty restaurant name")
		}
	}
}

func TestListRestaurantsFilterByOption(t *testing.T) {
	resp := get(t, "/api/restaurants?options=vegan", "")
	assertStatus(t, resp, http.StatusOK)
	var body struct {
		Restaurants []dbpkg.Restaurant `json:"restaurants"`
	}
	decodeJSON(t, resp, &body)
	for _, r := range body.Restaurants {
		found := false
		for _, o := range r.Options {
			if o == "vegan" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("restaurant %q missing vegan option", r.Name)
		}
	}
}

func TestGetRestaurant(t *testing.T) {
	// Get list first to pick a real ID
	resp := get(t, "/api/restaurants", "")
	var list struct {
		Restaurants []dbpkg.Restaurant `json:"restaurants"`
	}
	decodeJSON(t, resp, &list)
	if len(list.Restaurants) == 0 {
		t.Skip("no restaurants in DB")
	}
	first := list.Restaurants[0]

	resp = get(t, "/api/restaurants/"+first.Slug, aliceID)
	assertStatus(t, resp, http.StatusOK)
	var r dbpkg.Restaurant
	decodeJSON(t, resp, &r)
	if r.ID != first.ID {
		t.Errorf("expected id %s, got %s", first.ID, r.ID)
	}
	if r.Name == "" {
		t.Error("expected non-empty name")
	}
}

func TestCreateAndUpdateRestaurant(t *testing.T) {
	// Create
	body := map[string]any{
		"name":         "Test Tapas Bar",
		"neighborhood": "El Born",
		"address":      "Carrer de Test 1",
		"description":  "A test restaurant",
		"hours":        []any{map[string]string{"day": "monday", "open": "12:00", "close": "22:00"}},
		"options":      []string{"vegan"},
		"tapas_menu":   []any{map[string]any{"name": "Test Tapa", "price": 5.0, "options": []string{"vegan"}}},
	}
	resp := postJSON(t, "/api/restaurants", body, adminID)
	assertStatus(t, resp, http.StatusCreated)
	var created dbpkg.Restaurant
	decodeJSON(t, resp, &created)
	if created.ID == "" {
		t.Fatal("expected non-empty id")
	}
	if created.Name != "Test Tapas Bar" {
		t.Errorf("expected name Test Tapas Bar, got %s", created.Name)
	}

	// Update
	resp = putJSON(t, "/api/restaurants/"+created.Slug, map[string]string{"description": "Updated description"}, adminID)
	assertStatus(t, resp, http.StatusOK)
	var updated dbpkg.Restaurant
	decodeJSON(t, resp, &updated)
	if updated.Description != "Updated description" {
		t.Errorf("expected updated description, got %q", updated.Description)
	}

	// Cleanup
	req, _ := http.NewRequest(http.MethodDelete, testServer.URL+"/api/restaurants/"+created.Slug, nil)
	req.Header.Set("user-id", adminID)
	http.DefaultClient.Do(req)
}

func TestSubmitAndUpdateRating(t *testing.T) {
	// Use a seeded restaurant
	resp := get(t, "/api/restaurants", "")
	var list struct {
		Restaurants []dbpkg.Restaurant `json:"restaurants"`
	}
	decodeJSON(t, resp, &list)
	if len(list.Restaurants) == 0 {
		t.Skip("no restaurants in DB")
	}

	// Find one without alice's seed rating (use last one if possible)
	last := list.Restaurants[len(list.Restaurants)-1]
	rID := last.ID     // short hex ID, used for DB queries
	rSlug := last.Slug // slug, used for URL

	// Clear any existing rating from alice for this restaurant
	testDB.Exec(`DELETE FROM ratings WHERE user_id = $1 AND restaurant_id = $2`, aliceID, rID)

	// Submit rating
	resp = postJSON(t, "/api/restaurants/"+rSlug+"/ratings", map[string]int{"rating": 3}, aliceID)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200/201, got %d", resp.StatusCode)
	}
	var result map[string]any
	decodeJSON(t, resp, &result)
	if _, ok := result["new_avg_rating"]; !ok {
		t.Error("expected new_avg_rating in response")
	}

	// Update rating
	resp = postJSON(t, "/api/restaurants/"+rSlug+"/ratings", map[string]int{"rating": 5}, aliceID)
	assertStatus(t, resp, http.StatusOK)
	var result2 map[string]any
	decodeJSON(t, resp, &result2)
	if int(result2["rating"].(float64)) != 5 {
		t.Errorf("expected rating 5, got %v", result2["rating"])
	}

	// Cleanup
	testDB.Exec(`DELETE FROM ratings WHERE user_id = $1 AND restaurant_id = $2`, aliceID, rID)
}

func TestGetFavorites(t *testing.T) {
	resp := get(t, "/api/users/me/favorites", aliceID)
	assertStatus(t, resp, http.StatusOK)
	var body struct {
		Restaurants []dbpkg.Restaurant `json:"restaurants"`
	}
	decodeJSON(t, resp, &body)
	for _, r := range body.Restaurants {
		if r.ID == "" {
			t.Error("restaurant without ID in favorites")
		}
	}
}
