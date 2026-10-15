package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/redis"
)

type ActiveStreamsHandler struct {
	cooldown *redis.CooldownStore
}

func NewActiveStreamsHandler(cooldown *redis.CooldownStore) *ActiveStreamsHandler {
	return &ActiveStreamsHandler{cooldown: cooldown}
}

func (h *ActiveStreamsHandler) GetActiveStreams(c *gin.Context) {
	summary, err := h.cooldown.GetActiveStreams(c.Request.Context(), c.GetString("userId"))
	if err != nil {
		httputil.RespondInternalError(c, "Failed to fetch active streams", err, "ACTIVE_STREAMS_FETCH_FAILED")
		return
	}
	c.JSON(http.StatusOK, summary)
}
