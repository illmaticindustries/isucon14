package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/goccy/go-json"

	"github.com/bradfitz/gomemcache/memcache"
)

func appAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c, err := r.Cookie("app_session")
		if errors.Is(err, http.ErrNoCookie) || c.Value == "" {
			writeError(w, http.StatusUnauthorized, errors.New("app_session cookie is required"))
			return
		}
		accessToken := c.Value
		user := &User{}
		err = db.GetContext(ctx, user, "SELECT * FROM users WHERE access_token = ?", accessToken)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusUnauthorized, errors.New("invalid access token"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		ctx = context.WithValue(ctx, "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ownerAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c, err := r.Cookie("owner_session")
		if errors.Is(err, http.ErrNoCookie) || c.Value == "" {
			writeError(w, http.StatusUnauthorized, errors.New("owner_session cookie is required"))
			return
		}
		accessToken := c.Value
		owner := &Owner{}
		if err := db.GetContext(ctx, owner, "SELECT * FROM owners WHERE access_token = ?", accessToken); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusUnauthorized, errors.New("invalid access token"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		ctx = context.WithValue(ctx, "owner", owner)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// func chairAuthMiddleware(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		ctx := r.Context()
// 		c, err := r.Cookie("chair_session")
// 		if errors.Is(err, http.ErrNoCookie) || c.Value == "" {
// 			writeError(w, http.StatusUnauthorized, errors.New("chair_session cookie is required"))
// 			return
// 		}
// 		accessToken := c.Value
// 		chair := &Chair{}
// 		err = db.GetContext(ctx, chair, "SELECT * FROM chairs WHERE access_token = ?", accessToken)
// 		if err != nil {
// 			if errors.Is(err, sql.ErrNoRows) {
// 				writeError(w, http.StatusUnauthorized, errors.New("invalid access token"))
// 				return
// 			}
// 			writeError(w, http.StatusInternalServerError, err)
// 			return
// 		}

// 		ctx = context.WithValue(ctx, "chair", chair)
// 		next.ServeHTTP(w, r.WithContext(ctx))
// 	})
// }

// Middlewareの改善版
func chairAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Cookieの取得
		c, err := r.Cookie("chair_session")
		if err != nil || c.Value == "" {
			writeError(w, http.StatusUnauthorized, errors.New("chair_session cookie is required"))
			return
		}
		accessToken := c.Value

		// Memcachedから取得
		chair, found := cacheGetChair(accessToken)
		if !found {
			// キャッシュミス時はデータベースから取得
			chair = &Chair{}
			err := db.GetContext(ctx, chair, `
				SELECT id, name, model, is_active
				FROM chairs
				WHERE access_token = ?
			`, accessToken)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusUnauthorized, errors.New("invalid access token"))
					return
				}
				writeError(w, http.StatusInternalServerError, err)
				return
			}

			// キャッシュに保存
			cacheSetChair(accessToken, chair, 5*time.Minute)
		}

		// Contextにセットして次のハンドラを呼び出し
		ctx = context.WithValue(ctx, "chair", chair)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// キャッシュからChairを取得
func cacheGetChair(accessToken string) (*Chair, bool) {
	item, err := memcachedClient.Get(accessToken)
	if err != nil {
		if err == memcache.ErrCacheMiss {
			return nil, false
		}
		// その他のエラーはログ出力（必要に応じて実装）
		return nil, false
	}

	chair := &Chair{}
	if err := json.Unmarshal(item.Value, chair); err != nil {
		// JSONデコードエラーの場合はキャッシュを無視
		return nil, false
	}

	return chair, true
}

// キャッシュにChairを保存
func cacheSetChair(accessToken string, chair *Chair, duration time.Duration) {
	data, err := json.Marshal(chair)
	if err != nil {
		// JSONエンコードエラーは無視
		return
	}

	// Memcachedに保存
	memcachedClient.Set(&memcache.Item{
		Key:        accessToken,
		Value:      data,
		Expiration: int32(duration.Seconds()),
	})
}
