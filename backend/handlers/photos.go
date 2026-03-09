package handlers

import (
	"database/sql"
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

const maxPhotoSize = 10 << 20 // 10 MB

func UploadPhoto(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := mux.Vars(r)["id"]

		// Resolve slug → UUID and count existing photos in one query.
		var restaurantID string
		var photoCount int
		err := db.QueryRowContext(r.Context(), `
			SELECT r.id, COUNT(p.id)
			FROM restaurants r
			LEFT JOIN photos p ON p.restaurant_id = r.id
			WHERE r.slug = $1
			GROUP BY r.id`,
			slug,
		).Scan(&restaurantID, &photoCount)
		if err == sql.ErrNoRows {
			Err(w, http.StatusNotFound, "restaurant not found")
			return
		}
		if err != nil {
			Err(w, http.StatusInternalServerError, "database error")
			return
		}
		if photoCount >= 2 {
			Err(w, http.StatusConflict, "restaurant already has 2 photos")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxPhotoSize)
		if err := r.ParseMultipartForm(maxPhotoSize); err != nil {
			Err(w, http.StatusBadRequest, "could not parse form")
			return
		}
		file, header, err := r.FormFile("photo")
		if err != nil {
			Err(w, http.StatusBadRequest, "missing photo field")
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			Err(w, http.StatusInternalServerError, "could not read file")
			return
		}

		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "image/jpeg"
		}

		var photoID string
		err = db.QueryRowContext(r.Context(),
			`INSERT INTO photos (id, restaurant_id, data, content_type) VALUES ($1, $2, $3, $4) RETURNING id`,
			newID(), restaurantID, data, contentType,
		).Scan(&photoID)
		if err != nil {
			Err(w, http.StatusInternalServerError, "could not save photo")
			return
		}

		JSON(w, http.StatusCreated, map[string]string{"id": photoID})
	}
}

func GetPhoto(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		photoID := vars["photo_id"]
		slug := vars["id"]

		var data []byte
		var contentType string
		err := db.QueryRowContext(r.Context(),
			`SELECT data, content_type FROM photos
			 WHERE id = $1 AND restaurant_id = (SELECT id FROM restaurants WHERE slug = $2)`,
			photoID, slug,
		).Scan(&data, &contentType)
		if err == sql.ErrNoRows {
			Err(w, http.StatusNotFound, "photo not found")
			return
		}
		if err != nil {
			Err(w, http.StatusInternalServerError, "database error")
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func DeletePhoto(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		photoID := vars["photo_id"]
		slug := vars["id"]

		res, err := db.ExecContext(r.Context(),
			`DELETE FROM photos WHERE id = $1
			 AND restaurant_id = (SELECT id FROM restaurants WHERE slug = $2)`,
			photoID, slug)
		if err != nil {
			Err(w, http.StatusInternalServerError, "could not delete photo")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			Err(w, http.StatusNotFound, "photo not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
