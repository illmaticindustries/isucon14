package main

import (
	"database/sql"
	"errors"
	"net/http"
)

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// MEMO: 一旦最も待たせているリクエストに適当な空いている椅子マッチさせる実装とする。おそらくもっといい方法があるはず…
	ride := &Ride{}
	if err := db.GetContext(ctx, ride, `
	SELECT * FROM rides WHERE chair_id IS NULL ORDER BY created_at LIMIT 1`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// empty := false
	// for i := 0; i < 10; i++ {
	// 	if err := db.GetContext(ctx, matched, "SELECT * FROM chairs INNER JOIN (SELECT id FROM chairs WHERE is_active = TRUE ORDER BY RAND() LIMIT 1) AS tmp ON chairs.id = tmp.id LIMIT 1"); err != nil {
	// 		if errors.Is(err, sql.ErrNoRows) {
	// 			w.WriteHeader(http.StatusNoContent)
	// 			return
	// 		}
	// 		writeError(w, http.StatusInternalServerError, err)
	// 	}

	// 	if err := db.GetContext(ctx, &empty, `
	// 	SELECT COUNT(*) = 0 FROM
	// 		(
	// 		SELECT COUNT(chair_sent_at) = 6 AS completed
	// 		FROM ride_statuses
	// 		WHERE ride_id IN
	// 			(
	// 	 			SELECT id FROM rides WHERE chair_id = ?
	// 			)
	// 		GROUP BY ride_id
	// 		) is_completed
	// 	WHERE completed = FALSE
	// 	`, matched.ID); err != nil {
	// 		writeError(w, http.StatusInternalServerError, err)
	// 		return
	// 	}
	// 	if empty {
	// 		break
	// 	}
	// }

	tx, err := db.Beginx()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()

	var vacant_chair struct {
		ID string `db:"chair_id"`
	}
	if err := tx.SelectContext(ctx, &vacant_chair, "SELECT chair_id FROM vacant_chairs LIMIT 1 FOR UPDATE"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM vacant_chairs WHERE chair_id = ?", vacant_chair.ID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	tx.Commit()

	if _, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", vacant_chair.ID, ride.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// func internalGetMatching(w http.ResponseWriter, r *http.Request) {
// 	ctx := r.Context()

// 	// 1. 最も待たせているライドを取得
// 	ride := &Ride{}
// 	err := db.GetContext(ctx, ride, `
// 		SELECT id, user_id, pickup_latitude, pickup_longitude, destination_latitude, destination_longitude
// 		FROM rides
// 		WHERE chair_id IS NULL
// 		ORDER BY created_at
// 		LIMIT 1
// 	`)
// 	if err != nil {
// 		if errors.Is(err, sql.ErrNoRows) {
// 			w.WriteHeader(http.StatusNoContent)
// 			return
// 		}
// 		writeError(w, http.StatusInternalServerError, err)
// 		return
// 	}

// 	// 2. 空いている椅子を取得
// 	chair := &Chair{}
// 	err = db.GetContext(ctx, chair, `
// 	SELECT c.id, c.name, c.model
// 	FROM chairs AS c
// 	LEFT JOIN (
// 	    SELECT r.chair_id
// 	    FROM rides AS r
// 	    JOIN ride_statuses AS rs ON r.id = rs.ride_id
// 	    WHERE rs.chair_sent_at IS NOT NULL
// 	    GROUP BY r.chair_id
// 	    HAVING COUNT(rs.chair_sent_at) = 6
// 	) AS active_rides
// 	ON c.id = active_rides.chair_id
// 	WHERE active_rides.chair_id IS NULL AND c.is_active = TRUE
// 	LIMIT 1
// 	`)
// 	if err != nil {
// 		if errors.Is(err, sql.ErrNoRows) {
// 			w.WriteHeader(http.StatusNoContent)
// 			return
// 		}
// 		writeError(w, http.StatusInternalServerError, err)
// 		return
// 	}

// 	// 3. ライドと椅子をマッチング
// 	_, err = db.ExecContext(ctx, `
// 		UPDATE rides
// 		SET chair_id = ?
// 		WHERE id = ?
// 	`, chair.ID, ride.ID)
// 	if err != nil {
// 		writeError(w, http.StatusInternalServerError, err)
// 		return
// 	}

// 	w.WriteHeader(http.StatusNoContent)
// }
