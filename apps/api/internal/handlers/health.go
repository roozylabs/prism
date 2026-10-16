package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db  *sqlx.DB
	rdb *redis.Client
}

func NewHealthHandler(db *sqlx.DB, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, rdb: rdb}
}

// pingTimeout bounds each dependency check, so a hung database or Redis
// fails the probe instead of hanging it.
const pingTimeout = 2 * time.Second

type HealthResponse struct {
	Status   string `json:"status"`
	Version  string `json:"version"`
	Database string `json:"database"`
	Redis    string `json:"redis"`
}

// Check is the liveness probe (/health): HTTP 200 as long as the process
// serves requests, with the dependency state for information. Deploy gates
// and container health checks use Ready.
func (h *HealthHandler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), pingTimeout)
	defer cancel()
	resp := HealthResponse{
		Status:  "ok",
		Version: "0.4.0",
	}

	if h.db != nil && h.db.PingContext(ctx) == nil {
		resp.Database = "ok"
	} else {
		resp.Database = "degraded"
	}

	if h.rdb != nil && h.rdb.Ping(ctx).Err() == nil {
		resp.Redis = "ok"
	} else {
		resp.Redis = "degraded"
	}

	c.JSON(http.StatusOK, resp)
}

// Ready is the readiness probe (/ready): HTTP 200 when the database and
// Redis answer within pingTimeout, otherwise 503.
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), pingTimeout)
	defer cancel()
	resp := HealthResponse{
		Status:  "ok",
		Version: "0.4.0",
	}

	dbOK := true
	if h.db == nil || h.db.PingContext(ctx) != nil {
		resp.Database = "error"
		dbOK = false
	} else {
		resp.Database = "ok"
	}

	redisOK := true
	if h.rdb == nil || h.rdb.Ping(ctx).Err() != nil {
		resp.Redis = "error"
		redisOK = false
	} else {
		resp.Redis = "ok"
	}

	status := http.StatusOK
	if !dbOK || !redisOK {
		resp.Status = "degraded"
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, resp)
}
