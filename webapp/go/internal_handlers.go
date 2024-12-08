package main

import (
	"database/sql"
	"errors"
	"net/http"
)

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
// func internalGetMatching(w http.ResponseWriter, r *http.Request) {
// 	ctx := r.Context()
// 	// MEMO: 一旦最も待たせているリクエストに適当な空いている椅子マッチさせる実装とする。おそらくもっといい方法があるはず…
// 	ride := &Ride{}
// 	if err := db.GetContext(ctx, ride, `SELECT * FROM rides WHERE chair_id IS NULL ORDER BY created_at LIMIT 1`); err != nil {
// 		if errors.Is(err, sql.ErrNoRows) {
// 			w.WriteHeader(http.StatusNoContent)
// 			return
// 		}
// 		writeError(w, http.StatusInternalServerError, err)
// 		return
// 	}

// 	matched := &Chair{}
// 	empty := false
// 	for i := 0; i < 10; i++ {
// 		if err := db.GetContext(ctx, matched, "SELECT * FROM chairs INNER JOIN (SELECT id FROM chairs WHERE is_active = TRUE ORDER BY RAND() LIMIT 1) AS tmp ON chairs.id = tmp.id LIMIT 1"); err != nil {
// 			if errors.Is(err, sql.ErrNoRows) {
// 				w.WriteHeader(http.StatusNoContent)
// 				return
// 			}
// 			writeError(w, http.StatusInternalServerError, err)
// 		}

// 		if err := db.GetContext(ctx, &empty, "SELECT COUNT(*) = 0 FROM (SELECT COUNT(chair_sent_at) = 6 AS completed FROM ride_statuses WHERE ride_id IN (SELECT id FROM rides WHERE chair_id = ?) GROUP BY ride_id) is_completed WHERE completed = FALSE", matched.ID); err != nil {
// 			writeError(w, http.StatusInternalServerError, err)
// 			return
// 		}
// 		if empty {
// 			break
// 		}
// 	}
// 	if !empty {
// 		w.WriteHeader(http.StatusNoContent)
// 		return
// 	}

// 	if _, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", matched.ID, ride.ID); err != nil {
// 		writeError(w, http.StatusInternalServerError, err)
// 		return
// 	}

// 	w.WriteHeader(http.StatusNoContent)
// }

func internalGetMatching(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. 最も待たせているライドを取得
	ride := &Ride{}
	err := db.GetContext(ctx, ride, `
		SELECT id, created_at
		FROM rides
		WHERE chair_id IS NULL
		ORDER BY created_at
		LIMIT 1
	`)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// 2. 空いている椅子を取得
	chair := &Chair{}
	err = db.GetContext(ctx, chair, `
		SELECT c.id, c.name, c.model
		FROM chairs AS c
		LEFT JOIN (
			SELECT chair_id
			FROM rides
			WHERE chair_id IS NOT NULL
			GROUP BY chair_id
			HAVING COUNT(CASE WHEN chair_sent_at IS NULL THEN 1 END) > 0
		) AS active_rides
		ON c.id = active_rides.chair_id
		WHERE active_rides.chair_id IS NULL AND c.is_active = TRUE
		LIMIT 1
	`)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// 3. ライドと椅子をマッチング
	_, err = db.ExecContext(ctx, `
		UPDATE rides
		SET chair_id = ?
		WHERE id = ?
	`, chair.ID, ride.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
