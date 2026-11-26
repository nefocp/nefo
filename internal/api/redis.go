package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/nefocp/nefo/internal/redisui"
	"github.com/nefocp/nefo/internal/usererr"
)

// redisClient talks to the local Redis server. It holds no connection, only
// where Redis is; tests may replace it.
var redisClient = redisui.New()

// redisDB reads the database number from the "db" query parameter (0 when
// absent).
func redisDB(r *http.Request) (int, error) {
	v := r.URL.Query().Get("db")
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || n >= redisui.Databases {
		return 0, usererr.New("redis_invalid_db", v)
	}
	return n, nil
}

// handleRedisInfo returns Redis's version, memory use and keys per database.
func (s *Server) handleRedisInfo(w http.ResponseWriter, r *http.Request, a *authed) {
	info, err := redisClient.Info(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// handleRedisScan returns one page of keys: ?db=0&pattern=user:*&cursor=0.
func (s *Server) handleRedisScan(w http.ResponseWriter, r *http.Request, a *authed) {
	db, err := redisDB(r)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	q := r.URL.Query()
	res, err := redisClient.Scan(r.Context(), db, q.Get("pattern"), q.Get("cursor"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleRedisKey returns one key with its value: ?db=0&key=<id>, where the
// ID is the key in base64 as the scan returned it.
func (s *Server) handleRedisKey(w http.ResponseWriter, r *http.Request, a *authed) {
	db, err := redisDB(r)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	key, err := redisui.DecodeKey(r.URL.Query().Get("key"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	k, err := redisClient.Get(r.Context(), db, key)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, k)
}

// handleRedisDeleteKey deletes one key: {"db": 0, "key": "<id>"}.
func (s *Server) handleRedisDeleteKey(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		DB  int    `json:"db"`
		Key string `json:"key"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	key, err := redisui.DecodeKey(req.Key)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	if err := redisClient.Delete(r.Context(), req.DB, key); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "redis_key_deleted", clientIP(r),
		fmt.Sprintf("db%d %s", req.DB, truncate(strconv.QuoteToASCII(key), 200)))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleRedisFlush deletes every key of a database: {"db": 3, "confirm": "3"}.
func (s *Server) handleRedisFlush(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		DB      int    `json:"db"`
		Confirm string `json:"confirm"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := redisClient.Flush(r.Context(), req.DB, req.Confirm); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "redis_flushed", clientIP(r), fmt.Sprintf("db%d", req.DB))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
