package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/featureflags"
	"github.com/roozylabs/prism/internal/httputil"
)

// RequireFeature answers 404 for routes behind a flag that is off, as if
// they did not exist. Only flags switched by environment variable (not by
// plan) are meant for this.
func RequireFeature(flag featureflags.FlagKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !featureflags.GetManager().IsEnabled(flag, "") {
			httputil.RespondError(c, http.StatusNotFound, "Not found", nil, "FEATURE_DISABLED")
			return
		}
		c.Next()
	}
}
