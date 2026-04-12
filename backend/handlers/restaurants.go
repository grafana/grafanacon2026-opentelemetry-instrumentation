package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/lib/pq"
	"github.com/workshop/tapas-backend/chaos"
	dbpkg "github.com/workshop/tapas-backend/db"
	"github.com/workshop/tapas-backend/middleware"
)

const photoSubquery = `COALESCE(
		(SELECT json_agg(p.id ORDER BY p.created_at) FROM photos p WHERE p.restaurant_id = r.id),
		'[]'
	) AS photo_ids`

// listSelect is the column list for list queries (no tapas_menu).
const listSelect = `r.id, r.slug, r.name, r.address, r.neighborhood, r.description,
	r.hours, r.options, r.avg_rating, r.created_at, r.updated_at, ` + photoSubquery

// listSelectChaos omits the photo subquery so photo IDs can be fetched in N
// separate queries, demonstrating the N+1 pattern.
const listSelectChaos = `r.id, r.slug, r.name, r.address, r.neighborhood, r.description,
	r.hours, r.options, r.avg_rating, r.created_at, r.updated_at, '[]'::json AS photo_ids`

// detailSelect includes tapas_menu.
const detailSelect = `r.id, r.slug, r.name, r.address, r.neighborhood, r.description,
	r.hours, r.options, r.tapas_menu, r.avg_rating, r.created_at, r.updated_at, ` + photoSubquery

func slugify(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + 32)
		case r == ' ' || r == '-':
			b.WriteRune('_')
		default:
			switch r {
			case 'á', 'à', 'â', 'ä', 'ã':
				b.WriteRune('a')
			case 'é', 'è', 'ê', 'ë':
				b.WriteRune('e')
			case 'í', 'ì', 'î', 'ï':
				b.WriteRune('i')
			case 'ó', 'ò', 'ô', 'ö', 'õ':
				b.WriteRune('o')
			case 'ú', 'ù', 'û', 'ü':
				b.WriteRune('u')
			case 'ñ':
				b.WriteRune('n')
			case 'ç':
				b.WriteRune('c')
			}
		}
	}
	slug := b.String()
	for strings.Contains(slug, "__") {
		slug = strings.ReplaceAll(slug, "__", "_")
	}
	return strings.Trim(slug, "_")
}

func ListRestaurants(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var conditions []string
		var args []interface{}
		i := 1

		if search := q.Get("q"); search != "" {
			conditions = append(conditions,
				fmt.Sprintf("(r.name ILIKE $%d OR r.description ILIKE $%d)", i, i+1))
			like := "%" + search + "%"
			args = append(args, like, like)
			i += 2
		}
		if nb := q.Get("neighborhood"); nb != "" {
			conditions = append(conditions, fmt.Sprintf("LOWER(r.neighborhood) = LOWER($%d)", i))
			args = append(args, nb)
			i++
		}
		if opts := q.Get("options"); opts != "" {
			for _, opt := range strings.Split(opts, ",") {
				opt = strings.TrimSpace(opt)
				if opt != "" {
					conditions = append(conditions, fmt.Sprintf("$%d = ANY(r.options)", i))
					args = append(args, opt)
					i++
				}
			}
		}
		if minR := q.Get("min_rating"); minR != "" {
			if v, err := strconv.ParseFloat(minR, 64); err == nil {
				conditions = append(conditions, fmt.Sprintf("r.avg_rating >= $%d", i))
				args = append(args, v)
				i++
			}
		}
		if openAt := q.Get("open_at"); openAt != "" {
			// Filter restaurants with a hours entry covering the current weekday at openAt.
			// Weekday: Go time.Weekday 0=Sunday..6=Saturday → lowercase day name.
			weekday := strings.ToLower(time.Now().Weekday().String())
			conditions = append(conditions, fmt.Sprintf(`
				EXISTS (
					SELECT 1 FROM jsonb_array_elements(r.hours) h
					WHERE h->>'day' = $%d
					  AND h->>'open'  <= $%d
					  AND h->>'close' > $%d
				)`, i, i+1, i+2))
			args = append(args, weekday, openAt, openAt)
			i += 3
		}

		where := ""
		if len(conditions) > 0 {
			where = "WHERE " + strings.Join(conditions, " AND ")
		}

		sel := listSelect
		if chaos.Triggered() {
			// CHAOS: drop the photo subquery from the main SELECT so each
			// restaurant's photos are fetched in a dedicated query below.
			sel = listSelectChaos
		}
		query := fmt.Sprintf("SELECT %s FROM restaurants r %s ORDER BY r.avg_rating DESC", sel, where)
		rows, err := db.QueryContext(r.Context(), query, args...)
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

		// CHAOS: n+1 — one extra query per restaurant to fetch photo IDs,
		// replacing the efficient correlated subquery in listSelect.
		if chaos.Triggered() {
			for _, rest := range restaurants {
				photoRows, err := db.QueryContext(r.Context(),
					"SELECT id FROM photos WHERE restaurant_id = $1 ORDER BY created_at", rest.ID,
				)
				if err == nil {
					var ids []string
					for photoRows.Next() {
						var id string
						_ = photoRows.Scan(&id)
						ids = append(ids, id)
					}
					photoRows.Close()
					if ids == nil {
						ids = []string{}
					}
					rest.PhotoIDs = ids
				}
			}
		}

		JSON(w, http.StatusOK, map[string]any{
			"restaurants": restaurants,
			"total":       len(restaurants),
		})
	}
}

func GetRestaurant(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := mux.Vars(r)["id"]

		// CHAOS: execute a query referencing a non-existent column so the DB
		// returns an error that propagates all the way back to the browser.
		if chaos.Triggered() {
			var x string
			err := db.QueryRowContext(r.Context(),
				"SELECT r.nonexistent_col FROM restaurants r WHERE r.slug = $1", slug,
			).Scan(&x)
			if err != nil {
				Err(w, http.StatusInternalServerError, fmt.Sprintf("database error: %v", err))
				return
			}
		}

		row := db.QueryRowContext(r.Context(),
			fmt.Sprintf("SELECT %s FROM restaurants r WHERE r.slug = $1", detailSelect), slug)
		rest, err := dbpkg.ScanRestaurant(row, true)
		if err == sql.ErrNoRows {
			Err(w, http.StatusNotFound, "restaurant not found")
			return
		}
		if err != nil {
			Err(w, http.StatusInternalServerError, "database error")
			return
		}

		// Rating count + caller's rating in one query.
		u := middleware.GetUser(r)
		var userID sql.NullString
		if u != nil {
			userID = sql.NullString{String: u.ID, Valid: true}
		}
		var myRating sql.NullInt64
		_ = db.QueryRowContext(r.Context(),
			`SELECT COUNT(*), MAX(CASE WHEN user_id = $2 THEN rating END)
			 FROM ratings WHERE restaurant_id = $1`,
			rest.ID, userID,
		).Scan(&rest.RatingCount, &myRating)
		if myRating.Valid {
			v := int(myRating.Int64)
			rest.MyRating = &v
		}

		JSON(w, http.StatusOK, rest)
	}
}

type restaurantInput struct {
	Name         string          `json:"name"`
	Address      string          `json:"address"`
	Neighborhood string          `json:"neighborhood"`
	Description  string          `json:"description"`
	Hours        json.RawMessage `json:"hours"`
	Options      []string        `json:"options"`
	TapasMenu    json.RawMessage `json:"tapas_menu"`
}

func CreateRestaurant(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var inp restaurantInput
		if err := json.NewDecoder(r.Body).Decode(&inp); err != nil {
			Err(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if strings.TrimSpace(inp.Name) == "" {
			Err(w, http.StatusBadRequest, "name is required")
			return
		}

		hours := inp.Hours
		if hours == nil {
			hours = json.RawMessage("[]")
		}
		tapas := inp.TapasMenu
		if tapas == nil {
			tapas = json.RawMessage("[]")
		}
		options := inp.Options
		if options == nil {
			options = []string{}
		}

		var id string
		err := db.QueryRowContext(r.Context(), `
			INSERT INTO restaurants (id, name, slug, address, neighborhood, description, hours, options, tapas_menu)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id`,
			newID(), inp.Name, slugify(inp.Name), inp.Address, inp.Neighborhood, inp.Description,
			hours, pq.Array(options), tapas,
		).Scan(&id)
		if err != nil {
			Err(w, http.StatusInternalServerError, "could not create restaurant")
			return
		}

		row := db.QueryRowContext(r.Context(),
			fmt.Sprintf("SELECT %s FROM restaurants r WHERE r.id = $1", detailSelect), id)
		rest, err := dbpkg.ScanRestaurant(row, true)
		if err != nil {
			Err(w, http.StatusInternalServerError, "could not fetch restaurant")
			return
		}
		JSON(w, http.StatusCreated, rest)
	}
}

func UpdateRestaurant(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := mux.Vars(r)["id"]

		var inp restaurantInput
		if err := json.NewDecoder(r.Body).Decode(&inp); err != nil {
			Err(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		sets := []string{"updated_at = NOW()"}
		args := []interface{}{}
		i := 1

		if inp.Name != "" {
			sets = append(sets, fmt.Sprintf("name = $%d", i))
			args = append(args, inp.Name)
			i++
			sets = append(sets, fmt.Sprintf("slug = $%d", i))
			args = append(args, slugify(inp.Name))
			i++
		}
		if inp.Address != "" {
			sets = append(sets, fmt.Sprintf("address = $%d", i))
			args = append(args, inp.Address)
			i++
		}
		if inp.Neighborhood != "" {
			sets = append(sets, fmt.Sprintf("neighborhood = $%d", i))
			args = append(args, inp.Neighborhood)
			i++
		}
		if inp.Description != "" {
			sets = append(sets, fmt.Sprintf("description = $%d", i))
			args = append(args, inp.Description)
			i++
		}
		if inp.Hours != nil {
			sets = append(sets, fmt.Sprintf("hours = $%d", i))
			args = append(args, inp.Hours)
			i++
		}
		if inp.Options != nil {
			sets = append(sets, fmt.Sprintf("options = $%d", i))
			args = append(args, pq.Array(inp.Options))
			i++
		}
		if inp.TapasMenu != nil {
			sets = append(sets, fmt.Sprintf("tapas_menu = $%d", i))
			args = append(args, inp.TapasMenu)
			i++
		}

		args = append(args, slug)
		var restaurantID string
		err := db.QueryRowContext(r.Context(),
			fmt.Sprintf("UPDATE restaurants SET %s WHERE slug = $%d RETURNING id", strings.Join(sets, ", "), i),
			args...).Scan(&restaurantID)
		if err == sql.ErrNoRows {
			Err(w, http.StatusNotFound, "restaurant not found")
			return
		}
		if err != nil {
			Err(w, http.StatusInternalServerError, "could not update restaurant")
			return
		}

		row := db.QueryRowContext(r.Context(),
			fmt.Sprintf("SELECT %s FROM restaurants r WHERE r.id = $1", detailSelect), restaurantID)
		rest, err := dbpkg.ScanRestaurant(row, true)
		if err != nil {
			Err(w, http.StatusInternalServerError, "could not fetch restaurant")
			return
		}
		JSON(w, http.StatusOK, rest)
	}
}

func DeleteRestaurant(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := mux.Vars(r)["id"]
		res, err := db.ExecContext(r.Context(), `DELETE FROM restaurants WHERE slug = $1`, slug)
		if err != nil {
			Err(w, http.StatusInternalServerError, "could not delete restaurant")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			Err(w, http.StatusNotFound, "restaurant not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
