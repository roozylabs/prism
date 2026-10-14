package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeJobStore struct {
	jobs map[string]*queue.AsyncJob
}

func (f *fakeJobStore) EnqueueJob(_ context.Context, job *queue.AsyncJob) error {
	f.jobs[job.JobID] = job
	return nil
}

func (f *fakeJobStore) GetJob(_ context.Context, jobID string) (*queue.AsyncJob, error) {
	job, ok := f.jobs[jobID]
	if !ok {
		return nil, errors.New("job not found")
	}
	return job, nil
}

func newJobTestHandler() *GatewayHandler {
	store := &fakeJobStore{jobs: map[string]*queue.AsyncJob{
		"job_alice": {
			JobID:          "job_alice",
			UserID:         "user_alice",
			Status:         queue.StatusCompleted,
			Model:          "gpt-4o",
			TenantCtx:      models.TenantContext{OrgID: "org_a"},
			RequestPayload: &proxy.ProxyRequest{Model: "gpt-4o"},
		},
	}}
	return &GatewayHandler{jobQueue: store}
}

// sessionAuth mimics AuthMiddleware + TenantMiddleware on the protected route.
func sessionAuth(userID, orgID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("userId", userID)
		c.Set(middleware.TenantContextKey, models.TenantContext{OrgID: orgID})
	}
}

// gatewayAuth mimics GatewayAuthMiddleware + TenantMiddleware on the /v1 routes.
func gatewayAuth(userID, orgID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("gatewayKey", &models.GatewayAPIKey{ID: "key_" + userID, UserID: userID, OrgID: &orgID})
		c.Set(middleware.TenantContextKey, models.TenantContext{OrgID: orgID})
	}
}

func getJob(t *testing.T, h *GatewayHandler, auth gin.HandlerFunc, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/jobs/:jobId", auth, h.GetJobStatus)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func TestGetJobStatus_Ownership(t *testing.T) {
	h := newJobTestHandler()

	// The missing-job response body is the reference for every denied case.
	missing := getJob(t, h, sessionAuth("user_alice", "org_a"), "/jobs/job_does_not_exist")
	require.Equal(t, http.StatusNotFound, missing.Code)
	assert.Contains(t, missing.Body.String(), "Job not found or expired")

	tests := []struct {
		name   string
		auth   gin.HandlerFunc
		status int
	}{
		{"session owner", sessionAuth("user_alice", "org_a"), http.StatusOK},
		{"gateway key owner", gatewayAuth("user_alice", "org_a"), http.StatusOK},
		{"session other user", sessionAuth("user_bob", "org_a"), http.StatusNotFound},
		{"gateway key other user", gatewayAuth("user_bob", "org_a"), http.StatusNotFound},
		{"owner in another org", sessionAuth("user_alice", "org_b"), http.StatusNotFound},
		{"unauthenticated", func(c *gin.Context) {}, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := getJob(t, h, tt.auth, "/jobs/job_alice")
			require.Equal(t, tt.status, w.Code)
			if tt.status == http.StatusOK {
				var job queue.AsyncJob
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &job))
				assert.Equal(t, "job_alice", job.JobID)
				assert.Equal(t, "user_alice", job.UserID)
				return
			}
			// A foreign job must be indistinguishable from a missing one.
			assert.Equal(t, missing.Body.String(), w.Body.String())
		})
	}
}

func TestAsyncChatCompletions_JobIDIgnoresRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &fakeJobStore{jobs: map[string]*queue.AsyncJob{}}
	h := &GatewayHandler{jobQueue: store}

	r := gin.New()
	r.POST("/chat/completions/async", gatewayAuth("user_alice", "org_a"), h.AsyncChatCompletions)

	ids := map[string]bool{}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/chat/completions/async",
			strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", "fixed-request-id")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		jobID, _ := resp["job_id"].(string)
		assert.True(t, strings.HasPrefix(jobID, "job_"))
		ids[jobID] = true
		assert.Equal(t, "user_alice", store.jobs[jobID].UserID)
	}
	assert.Len(t, ids, 2, "same X-Request-ID must not yield the same job ID")
}
