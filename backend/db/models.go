package db

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

type HourEntry struct {
	Day   string `json:"day"`
	Open  string `json:"open"`
	Close string `json:"close"`
}

type TapaItem struct {
	Name    string   `json:"name"`
	Price   float64  `json:"price"`
	Options []string `json:"options"`
}

type Restaurant struct {
	ID           string      `json:"id"`
	Slug         string      `json:"slug"`
	Name         string      `json:"name"`
	Address      string      `json:"address"`
	Neighborhood string      `json:"neighborhood"`
	Description  string      `json:"description"`
	Hours        []HourEntry `json:"hours"`
	Options      []string    `json:"options"`
	TapasMenu    []TapaItem  `json:"tapas_menu,omitempty"`
	AvgRating    float64     `json:"avg_rating"`
	RatingCount  int         `json:"rating_count,omitempty"`
	PhotoIDs     []string    `json:"photo_ids"`
	MyRating     *int        `json:"my_rating,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

func ScanRestaurant(row interface {
	Scan(dest ...any) error
}, includeTapas bool) (*Restaurant, error) {
	var r Restaurant
	var hoursJSON, tapasJSON []byte
	var options pq.StringArray
	var photoIDsJSON []byte

	if includeTapas {
		err := row.Scan(
			&r.ID, &r.Slug, &r.Name, &r.Address, &r.Neighborhood, &r.Description,
			&hoursJSON, &options, &tapasJSON, &r.AvgRating,
			&r.CreatedAt, &r.UpdatedAt, &photoIDsJSON,
		)
		if err != nil {
			return nil, err
		}
	} else {
		err := row.Scan(
			&r.ID, &r.Slug, &r.Name, &r.Address, &r.Neighborhood, &r.Description,
			&hoursJSON, &options, &r.AvgRating,
			&r.CreatedAt, &r.UpdatedAt, &photoIDsJSON,
		)
		if err != nil {
			return nil, err
		}
	}

	r.Options = []string(options)
	if r.Options == nil {
		r.Options = []string{}
	}

	if err := json.Unmarshal(hoursJSON, &r.Hours); err != nil {
		r.Hours = []HourEntry{}
	}
	if includeTapas && tapasJSON != nil {
		if err := json.Unmarshal(tapasJSON, &r.TapasMenu); err != nil {
			r.TapasMenu = []TapaItem{}
		}
	}

	if photoIDsJSON != nil {
		if err := json.Unmarshal(photoIDsJSON, &r.PhotoIDs); err != nil {
			r.PhotoIDs = []string{}
		}
	}
	if r.PhotoIDs == nil {
		r.PhotoIDs = []string{}
	}

	return &r, nil
}
