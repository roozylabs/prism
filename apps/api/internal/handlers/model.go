package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
)

type ModelHandler struct {
	models        *repository.ModelRepository
	providers     *repository.ProviderRepository
	gatewayKeys   *repository.GatewayKeyRepository
	cooldownStore *goredis.CooldownStore
}

func NewModelHandler(
	models *repository.ModelRepository,
	providers *repository.ProviderRepository,
	gatewayKeys *repository.GatewayKeyRepository,
	cooldownStore *goredis.CooldownStore,
) *ModelHandler {
	return &ModelHandler{
		models:        models,
		providers:     providers,
		gatewayKeys:   gatewayKeys,
		cooldownStore: cooldownStore,
	}
}

// List godoc
// @Summary      List models
// @Description  Get all models for a provider
// @Tags         models
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Success      200 {array} models.Model
// @Failure      500 {object} map[string]string
// @Router       /api/providers/{id}/models [get]
func (h *ModelHandler) List(c *gin.Context) {
	providerID := c.Param("id")
	search := c.Query("search")

	limit := 10
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	} else if l := c.Query("pageSize"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	page := 1
	if p := c.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}

	offset := (page - 1) * limit

	modelList, total, err := h.models.ListWithFilter(c.Request.Context(), providerID, search, limit, offset)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list models", err, "MODELS_LIST_FAILED")
		return
	}
	if modelList == nil {
		modelList = []models.Model{}
	}
	c.JSON(http.StatusOK, gin.H{
		"data":     modelList,
		"total":    total,
		"page":     page,
		"pageSize": limit,
	})
}

// Get godoc
// @Summary      Get model
// @Description  Get a model by ID
// @Tags         models
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Param        modelId path string true "Model ID"
// @Success      200 {object} models.Model
// @Failure      404 {object} map[string]string
// @Router       /api/providers/{id}/models/{modelId} [get]
func (h *ModelHandler) Get(c *gin.Context) {
	modelID := c.Param("modelId")
	m, err := h.models.FindByID(c.Request.Context(), modelID)
	if err != nil {
		httputil.RespondNotFound(c, "Model not found", err, "MODEL_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, m)
}

// Create godoc
// @Summary      Create model
// @Description  Create a new model for a provider
// @Tags         models
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Param        request body models.Model true "Model data"
// @Success      201 {object} models.Model
// @Failure      400 {object} map[string]string
// @Router       /api/providers/{id}/models [post]
func (h *ModelHandler) Create(c *gin.Context) {
	providerID := c.Param("id")
	var m models.Model
	if err := c.ShouldBindJSON(&m); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	m.ProviderID = providerID

	if err := h.models.Create(c.Request.Context(), &m); err != nil {
		httputil.RespondInternalError(c, "Failed to create model", err, "MODEL_CREATE_FAILED")
		return
	}
	c.JSON(http.StatusCreated, m)
}

// Update godoc
// @Summary      Update model
// @Description  Update a model
// @Tags         models
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Param        modelId path string true "Model ID"
// @Param        request body models.Model true "Model data"
// @Success      200 {object} models.Model
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /api/providers/{id}/models/{modelId} [put]
func (h *ModelHandler) Update(c *gin.Context) {
	modelID := c.Param("modelId")
	existing, err := h.models.FindByID(c.Request.Context(), modelID)
	if err != nil {
		httputil.RespondNotFound(c, "Model not found", err, "MODEL_NOT_FOUND")
		return
	}

	if err := c.ShouldBindJSON(existing); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	if err := h.models.Update(c.Request.Context(), existing); err != nil {
		httputil.RespondInternalError(c, "Failed to update model", err, "MODEL_UPDATE_FAILED")
		return
	}
	c.JSON(http.StatusOK, existing)
}

// Delete godoc
// @Summary      Delete model
// @Description  Delete a model
// @Tags         models
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Param        modelId path string true "Model ID"
// @Success      204
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /api/providers/{id}/models/{modelId} [delete]
func (h *ModelHandler) Delete(c *gin.Context) {
	modelID := c.Param("modelId")
	m, err := h.models.FindByID(c.Request.Context(), modelID)
	if err != nil {
		httputil.RespondNotFound(c, "Model not found", err, "MODEL_NOT_FOUND")
		return
	}

	// 1. Guard against deleting model with active in-flight streams
	if h.cooldownStore != nil {
		if summary, err := h.cooldownStore.GetActiveStreams(c.Request.Context(), ""); err == nil {
			count := summary.ByModel[m.Slug] + summary.ByModel[m.Name]
			if count > 0 {
				httputil.RespondError(c, http.StatusConflict, fmt.Sprintf("Cannot delete model: it is currently processing %d active live streams", count), nil, "MODEL_IN_USE")
				return
			}
		}
	}

	// 2. Guard against deleting model referenced in active Gateway API Keys
	if h.gatewayKeys != nil {
		keyCount, err := h.gatewayKeys.CountByAllowedModel(c.Request.Context(), m.Slug)
		if err == nil && keyCount > 0 {
			httputil.RespondError(c, http.StatusConflict, fmt.Sprintf("Cannot delete model: in use by %d active Gateway API Key(s)", keyCount), nil, "MODEL_IN_USE_BY_KEYS")
			return
		}
		if m.Name != m.Slug {
			keyCount2, err := h.gatewayKeys.CountByAllowedModel(c.Request.Context(), m.Name)
			if err == nil && keyCount2 > 0 {
				httputil.RespondError(c, http.StatusConflict, fmt.Sprintf("Cannot delete model: in use by %d active Gateway API Key(s)", keyCount2), nil, "MODEL_IN_USE_BY_KEYS")
				return
			}
		}
	}

	if err := h.models.Delete(c.Request.Context(), modelID); err != nil {
		httputil.RespondInternalError(c, "Failed to delete model", err, "MODEL_DELETE_FAILED")
		return
	}
	c.Status(http.StatusNoContent)
}

type UpdateCapabilitiesRequest struct {
	ContextWindow    *int     `json:"contextWindow"`
	CodingScore      *float64 `json:"codingScore"`
	ReasoningScore   *float64 `json:"reasoningScore"`
	WritingScore     *float64 `json:"writingScore"`
	SpeedScore       *float64 `json:"speedScore"`
	QualityScore     *float64 `json:"qualityScore"`
	InputPricePer1M  *float64 `json:"inputPricePer1M"`
	OutputPricePer1M *float64 `json:"outputPricePer1M"`
	SupportsTools    *bool    `json:"supportsTools"`
	SupportsVision   *bool    `json:"supportsVision"`
}

// UpdateCapabilities godoc
// @Summary      Update model capabilities
// @Description  Update model capability scores, pricing, and features
// @Tags         models
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Param        modelId path string true "Model ID"
// @Param        request body UpdateCapabilitiesRequest true "Capability data"
// @Success      200 {object} models.Model
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /api/providers/{id}/models/{modelId}/capabilities [patch]
func (h *ModelHandler) UpdateCapabilities(c *gin.Context) {
	modelID := c.Param("modelId")
	m, err := h.models.FindByID(c.Request.Context(), modelID)
	if err != nil {
		httputil.RespondNotFound(c, "Model not found", err, "MODEL_NOT_FOUND")
		return
	}

	var req UpdateCapabilitiesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	if req.ContextWindow != nil {
		m.ContextWindow = *req.ContextWindow
	}
	if req.CodingScore != nil {
		m.CodingScore = *req.CodingScore
	}
	if req.ReasoningScore != nil {
		m.ReasoningScore = *req.ReasoningScore
	}
	if req.WritingScore != nil {
		m.WritingScore = *req.WritingScore
	}
	if req.SpeedScore != nil {
		m.SpeedScore = *req.SpeedScore
	}
	if req.QualityScore != nil {
		m.QualityScore = *req.QualityScore
	}
	if req.InputPricePer1M != nil {
		m.InputPricePer1M = *req.InputPricePer1M
	}
	if req.OutputPricePer1M != nil {
		m.OutputPricePer1M = *req.OutputPricePer1M
	}
	if req.SupportsTools != nil {
		m.SupportsTools = *req.SupportsTools
	}
	if req.SupportsVision != nil {
		m.SupportsVision = *req.SupportsVision
	}

	if err := h.models.Update(c.Request.Context(), m); err != nil {
		httputil.RespondInternalError(c, "Failed to update capabilities", err, "CAPABILITIES_UPDATE_FAILED")
		return
	}
	c.JSON(http.StatusOK, m)
}
