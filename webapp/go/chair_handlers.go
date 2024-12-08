package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"
)

type chairPostChairsRequest struct {
	Name               string `json:"name"`
	Model              string `json:"model"`
	ChairRegisterToken string `json:"chair_register_token"`
}

type chairPostChairsResponse struct {
	ID      string `json:"id"`
	OwnerID string `json:"owner_id"`
}

func chairPostChairs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &chairPostChairsRequest{}
	if err := bindJSON(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Model == "" || req.ChairRegisterToken == "" {
		writeError(w, http.StatusBadRequest, errors.New("some of required fields(name, model, chair_register_token) are empty"))
		return
	}

	owner := &Owner{}
	if err := db.GetContext(ctx, owner, "SELECT * FROM owners WHERE chair_register_token = ?", req.ChairRegisterToken); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, errors.New("invalid chair_register_token"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	chairID := ulid.Make().String()
	accessToken := secureRandomStr(32)

	_, err := db.ExecContext(
		ctx,
		"INSERT INTO chairs (id, owner_id, name, model, is_active, access_token) VALUES (?, ?, ?, ?, ?, ?)",
		chairID, owner.ID, req.Name, req.Model, false, accessToken,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Path:  "/",
		Name:  "chair_session",
		Value: accessToken,
	})

	writeJSON(w, http.StatusCreated, &chairPostChairsResponse{
		ID:      chairID,
		OwnerID: owner.ID,
	})
}

type postChairActivityRequest struct {
	IsActive bool `json:"is_active"`
}

func chairPostActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	chair := ctx.Value("chair").(*Chair)

	req := &postChairActivityRequest{}
	if err := bindJSON(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	_, err := db.ExecContext(ctx, "UPDATE chairs SET is_active = ? WHERE id = ?", req.IsActive, chair.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type chairPostCoordinateResponse struct {
	RecordedAt int64 `json:"recorded_at"`
}

func chairPostCoordinate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &Coordinate{}
	if err := bindJSON(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	chair := ctx.Value("chair").(*Chair)

	tx, err := db.Beginx()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()

	// 座標を挿入
	chairLocationID := ulid.Make().String()
	if _, err := tx.ExecContext(ctx, `INSERT INTO chair_locations (id, chair_id, latitude, longitude) VALUES (?, ?, ?, ?)`, chairLocationID, chair.ID, req.Latitude, req.Longitude); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// 直近のライドとステータスを取得
	var ride struct {
		ID                 string `db:"id"`
		PickupLatitude     int    `db:"pickup_latitude"`
		PickupLongitude    int    `db:"pickup_longitude"`
		DestinationLatitude int   `db:"destination_latitude"`
		DestinationLongitude int  `db:"destination_longitude"`
		Status             string `db:"status"`
	}
	err = tx.GetContext(ctx, &ride, `
		SELECT rides.id, rides.pickup_latitude, rides.pickup_longitude, rides.destination_latitude, rides.destination_longitude, rs.status
		FROM rides
		LEFT JOIN (
			SELECT ride_id, status
			FROM ride_statuses
			ORDER BY created_at DESC
			LIMIT 1
		) rs ON rides.id = rs.ride_id
		WHERE rides.chair_id = ?
		ORDER BY rides.updated_at DESC
		LIMIT 1`, chair.ID)

	// ステータス更新処理
	if err == nil && ride.Status != "COMPLETED" && ride.Status != "CANCELED" {
		newStatus := ""
		if req.Latitude == ride.PickupLatitude && req.Longitude == ride.PickupLongitude && ride.Status == "ENROUTE" {
			newStatus = "PICKUP"
		} else if req.Latitude == ride.DestinationLatitude && req.Longitude == ride.DestinationLongitude && ride.Status == "CARRYING" {
			newStatus = "ARRIVED"
		}
		if newStatus != "" {
			go func() {
				_, _ = db.ExecContext(ctx, "INSERT INTO ride_statuses (id, ride_id, status) VALUES (?, ?, ?)", ulid.Make().String(), ride.ID, newStatus)
			}()
		}
	}

	// コミット
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, &chairPostCoordinateResponse{
		RecordedAt: time.Now().UnixMilli(),
	})
}

type simpleUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type chairGetNotificationResponse struct {
	Data         *chairGetNotificationResponseData `json:"data"`
	RetryAfterMs int                               `json:"retry_after_ms"`
}

type chairGetNotificationResponseData struct {
	RideID                string     `json:"ride_id"`
	User                  simpleUser `json:"user"`
	PickupCoordinate      Coordinate `json:"pickup_coordinate"`
	DestinationCoordinate Coordinate `json:"destination_coordinate"`
	Status                string     `json:"status"`
}

func chairGetNotification(w http.ResponseWriter, r *http.Request) {

    ctx := r.Context()
    chair := ctx.Value("chair").(*Chair)

    tx, err := db.Beginx()
    if err != nil {
        writeError(w, http.StatusInternalServerError, err)
        return
    }
    defer tx.Rollback()

    var result struct {
        RideID       string `db:"id"`
        RideStatus   string `db:"ride_status"`
        RideStatusID string `db:"ride_status_id"`
        UserID       string `db:"user_id"`
        PickupLat    int    `db:"pickup_latitude"`
        PickupLon    int    `db:"pickup_longitude"`
        DestLat      int    `db:"destination_latitude"`
        DestLon      int    `db:"destination_longitude"`
        Firstname    string `db:"firstname"`
        Lastname     string `db:"lastname"`
    }

    // 統合クエリで必要なデータを一度に取得
    err = tx.GetContext(ctx, &result, `
        SELECT r.id, rs.status AS ride_status, rs.id AS ride_status_id, r.user_id, 
               r.pickup_latitude, r.pickup_longitude, r.destination_latitude, r.destination_longitude, 
               u.firstname, u.lastname
        FROM rides r
        LEFT JOIN ride_statuses rs ON r.id = rs.ride_id AND rs.chair_sent_at IS NULL
        JOIN users u ON r.user_id = u.id
        WHERE r.chair_id = ?
        ORDER BY r.updated_at DESC, rs.created_at ASC
        LIMIT 1
    `, chair.ID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            writeJSON(w, http.StatusOK, &chairGetNotificationResponse{
                RetryAfterMs: 30,
            })
            return
        }
        writeError(w, http.StatusInternalServerError, err)
        return
    }

    // 必要な場合にのみステータスを更新
    if result.RideStatusID != "" {
        _, err = tx.ExecContext(ctx, `
            UPDATE ride_statuses 
            SET chair_sent_at = CURRENT_TIMESTAMP(6) 
            WHERE id = ?
        `, result.RideStatusID)
        if err != nil {
            writeError(w, http.StatusInternalServerError, err)
            return
        }
    }

    if err := tx.Commit(); err != nil {
        writeError(w, http.StatusInternalServerError, err)
        return
    }

    // レスポンス作成
    writeJSON(w, http.StatusOK, &chairGetNotificationResponse{
        Data: &chairGetNotificationResponseData{
            RideID: result.RideID,
            User: simpleUser{
                ID:   result.UserID,
                Name: fmt.Sprintf("%s %s", result.Firstname, result.Lastname),
            },
            PickupCoordinate: Coordinate{
                Latitude:  result.PickupLat,
                Longitude: result.PickupLon,
            },
            DestinationCoordinate: Coordinate{
                Latitude:  result.DestLat,
                Longitude: result.DestLon,
            },
            Status: result.RideStatus,
        },
        RetryAfterMs: 30,
    })
}

type postChairRidesRideIDStatusRequest struct {
	Status string `json:"status"`
}

func chairPostRideStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rideID := r.PathValue("ride_id")

	chair := ctx.Value("chair").(*Chair)

	req := &postChairRidesRideIDStatusRequest{}
	if err := bindJSON(r, req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	tx, err := db.Beginx()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()

	ride := &Ride{}
	if err := tx.GetContext(ctx, ride, "SELECT * FROM rides WHERE id = ? FOR UPDATE", rideID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("ride not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if ride.ChairID.String != chair.ID {
		writeError(w, http.StatusBadRequest, errors.New("not assigned to this ride"))
		return
	}

	switch req.Status {
	// Acknowledge the ride
	case "ENROUTE":
		if _, err := tx.ExecContext(ctx, "INSERT INTO ride_statuses (id, ride_id, status) VALUES (?, ?, ?)", ulid.Make().String(), ride.ID, "ENROUTE"); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	// After Picking up user
	case "CARRYING":
		status, err := getLatestRideStatus(ctx, tx, ride.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if status != "PICKUP" {
			writeError(w, http.StatusBadRequest, errors.New("chair has not arrived yet"))
			return
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO ride_statuses (id, ride_id, status) VALUES (?, ?, ?)", ulid.Make().String(), ride.ID, "CARRYING"); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, errors.New("invalid status"))
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
