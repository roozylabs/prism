package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	_ "github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/service"
	"github.com/roozylabs/prism/internal/utils"
)

// AuthAttemptLimiter throttles sign-in and sign-up attempts per key.
type AuthAttemptLimiter interface {
	Hit(ctx context.Context, key string) (bool, time.Duration, error)
	Blocked(ctx context.Context, key string) (bool, time.Duration, error)
	Reset(ctx context.Context, key string) error
}

type AuthHandler struct {
	auth      *service.AuthService
	turnstile *Turnstile
	// perIP limits sign-in and sign-up requests from one client address;
	// perEmail limits failed sign-ins to one account from anywhere.
	perIP, perEmail AuthAttemptLimiter
}

func NewAuthHandler(auth *service.AuthService, turnstile *Turnstile, perIP, perEmail AuthAttemptLimiter) *AuthHandler {
	return &AuthHandler{auth: auth, turnstile: turnstile, perIP: perIP, perEmail: perEmail}
}

func (h *AuthHandler) GetTurnstileConfig(c *gin.Context) {
	siteKey := os.Getenv("CLOUDFLARE_SITE_KEY")
	if siteKey == "" {
		siteKey = os.Getenv("NEXT_PUBLIC_CLOUDFLARE_SITE_KEY")
	}
	enabled := siteKey != "" && h.turnstile.Enabled()

	c.JSON(http.StatusOK, gin.H{
		"enabled": enabled,
		"siteKey": siteKey,
	})
}

// cloudflareTestTokens are the dummy responses Cloudflare's test site keys
// return. They prove nothing, so they pass only in development.
var cloudflareTestTokens = map[string]bool{
	"10000000-aaaa-bbbb-cccc-000000000001": true,
	"XXXX.DUMMY.TOKEN.XXXX":                true,
}

// Turnstile verifies Cloudflare Turnstile tokens. With no secret it is off
// and every request passes.
type Turnstile struct {
	secret          string
	allowTestTokens bool
	client          *http.Client
	verifyURL       string
}

func NewTurnstile(secret string, allowTestTokens bool) *Turnstile {
	secret = strings.Trim(strings.TrimSpace(secret), `"'`)
	return &Turnstile{
		secret:          secret,
		allowTestTokens: allowTestTokens,
		client:          &http.Client{Timeout: 5 * time.Second},
		verifyURL:       "https://challenges.cloudflare.com/turnstile/v0/siteverify",
	}
}

func (t *Turnstile) Enabled() bool { return t != nil && t.secret != "" }

type turnstileResponse struct {
	OK         bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify reports whether token is a valid Turnstile response, and a short
// reason when it is not.
func (t *Turnstile) Verify(ctx context.Context, token, remoteIP string) (bool, string) {
	if !t.Enabled() {
		return true, "disabled"
	}
	token = strings.Trim(strings.TrimSpace(token), `"'`)
	if token == "" {
		return false, "missing-turnstile-token"
	}
	if cloudflareTestTokens[token] {
		if t.allowTestTokens {
			return true, "test-token"
		}
		return false, "test-token-rejected"
	}

	data := url.Values{}
	data.Set("secret", t.secret)
	data.Set("response", token)
	if remoteIP != "" {
		data.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.verifyURL, strings.NewReader(data.Encode()))
	if err != nil {
		return false, "request-error"
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err != nil {
		log.Printf("[Turnstile] verification request failed: %v", err)
		return false, "network-error"
	}
	defer func() { _ = resp.Body.Close() }()

	var result turnstileResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&result); err != nil {
		log.Printf("[Turnstile] unreadable verification response: %v", err)
		return false, "parse-error"
	}
	if !result.OK {
		detail := "failed"
		if len(result.ErrorCodes) > 0 {
			detail = strings.Join(result.ErrorCodes, ", ")
		}
		return false, detail
	}
	return true, "success"
}

// allowAttempt counts a sign-in or sign-up attempt from the client address
// and responds 429 when there have been too many. Limiter errors let the
// attempt through: Redis being down must not lock everyone out.
func (h *AuthHandler) allowAttempt(c *gin.Context) bool {
	if h.perIP == nil {
		return true
	}
	ok, retry, err := h.perIP.Hit(c.Request.Context(), c.ClientIP())
	if err != nil {
		log.Printf("[Auth] attempt limiter unavailable: %v", err)
		return true
	}
	if !ok {
		respondTooManyAttempts(c, retry)
		return false
	}
	return true
}

func respondTooManyAttempts(c *gin.Context, retry time.Duration) {
	c.Header("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
	httputil.RespondError(c, http.StatusTooManyRequests, "Too many attempts. Try again later.", nil, "AUTH_TOO_MANY_ATTEMPTS")
}

// emailKey keeps addresses out of Redis.
func emailKey(email string) string {
	return utils.HashSHA256(strings.TrimSpace(strings.ToLower(email)))
}

// Login godoc
// @Summary      Login
// @Description  Authenticate user with email and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body service.LoginRequest true "Login credentials"
// @Success      200 {object} service.LoginResponse
// @Failure      401 {object} map[string]string
// @Failure      429 {object} map[string]string
// @Router       /api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	if !h.allowAttempt(c) {
		return
	}
	if ok, detail := h.turnstile.Verify(c.Request.Context(), req.TurnstileToken, c.ClientIP()); !ok {
		httputil.RespondForbidden(c, "Security verification failed (Cloudflare Turnstile: "+detail+")", nil, "TURNSTILE_VERIFICATION_FAILED")
		return
	}

	// Failed sign-ins are counted per account, so guessing one password
	// from many addresses is throttled too.
	account := emailKey(req.Email)
	if h.perEmail != nil {
		if blocked, retry, err := h.perEmail.Blocked(c.Request.Context(), account); err == nil && blocked {
			respondTooManyAttempts(c, retry)
			return
		}
	}

	resp, err := h.auth.Login(
		c.Request.Context(),
		req.Email,
		req.Password,
		c.ClientIP(),
		c.Request.UserAgent(),
	)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			if h.perEmail != nil {
				_, _, _ = h.perEmail.Hit(c.Request.Context(), account)
			}
			httputil.RespondUnauthorized(c, "Invalid email or password", err, "AUTH_INVALID_CREDENTIALS")
			return
		}
		httputil.RespondInternalError(c, "Internal server error during login", err, "AUTH_LOGIN_FAILED")
		return
	}
	if h.perEmail != nil {
		_ = h.perEmail.Reset(c.Request.Context(), account)
	}

	// Set HttpOnly, SameSite=Lax cookie for browser session security
	c.SetSameSite(http.SameSiteLaxMode)
	isSecure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetCookie("auth_token", resp.Token, 7*24*3600, "/", "", isSecure, true)

	c.JSON(http.StatusOK, resp)
}

// Signup godoc
// @Summary      Signup
// @Description  Create new user account with email and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body service.SignupRequest true "Signup credentials"
// @Success      201 {object} service.LoginResponse
// @Failure      400 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Failure      429 {object} map[string]string
// @Router       /api/auth/signup [post]
func (h *AuthHandler) Signup(c *gin.Context) {
	var req service.SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	if !h.allowAttempt(c) {
		return
	}
	if ok, detail := h.turnstile.Verify(c.Request.Context(), req.TurnstileToken, c.ClientIP()); !ok {
		httputil.RespondForbidden(c, "Security verification failed (Cloudflare Turnstile: "+detail+")", nil, "TURNSTILE_VERIFICATION_FAILED")
		return
	}

	resp, err := h.auth.Signup(
		c.Request.Context(),
		req.Name,
		req.Email,
		req.Password,
		c.ClientIP(),
		c.Request.UserAgent(),
	)
	if err != nil {
		var invalid *service.InvalidSignupError
		if errors.As(err, &invalid) {
			httputil.RespondBadRequest(c, invalid.Error(), err, "AUTH_INVALID_SIGNUP")
			return
		}
		if errors.Is(err, service.ErrEmailAlreadyExists) {
			httputil.RespondError(c, http.StatusConflict, "An account with this email address already exists", err, "AUTH_ACCOUNT_EXISTS")
			return
		}
		httputil.RespondInternalError(c, "Failed to create account", err, "AUTH_SIGNUP_FAILED")
		return
	}

	// Set HttpOnly, SameSite=Lax cookie for browser session security
	c.SetSameSite(http.SameSiteLaxMode)
	isSecure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetCookie("auth_token", resp.Token, 7*24*3600, "/", "", isSecure, true)

	c.JSON(http.StatusCreated, resp)
}

// Logout godoc
// @Summary      Logout
// @Description  Invalidate session token
// @Tags         auth
// @Security     BearerAuth
// @Success      200 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Router       /api/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	token := c.GetString("token")
	if token == "" {
		if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
			token = cookie
		}
	}
	if token == "" {
		httputil.RespondUnauthorized(c, "Unauthorized", nil, "AUTH_REQUIRED")
		return
	}

	if err := h.auth.Logout(c.Request.Context(), token); err != nil {
		httputil.RespondInternalError(c, "Internal server error during logout", err, "AUTH_LOGOUT_FAILED")
		return
	}

	// Clear HttpOnly cookie
	c.SetSameSite(http.SameSiteLaxMode)
	isSecure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetCookie("auth_token", "", -1, "/", "", isSecure, true)

	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// Me godoc
// @Summary      Get current user
// @Description  Get the currently authenticated user
// @Tags         auth
// @Security     BearerAuth
// @Success      200 {object} models.User
// @Failure      401 {object} map[string]string
// @Router       /api/auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	token := c.GetString("token")
	if token == "" {
		httputil.RespondUnauthorized(c, "Unauthorized", nil, "AUTH_REQUIRED")
		return
	}

	user, err := h.auth.Me(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) || errors.Is(err, service.ErrSessionExpired) {
			httputil.RespondUnauthorized(c, "Session expired or invalid", err, "AUTH_SESSION_EXPIRED")
			return
		}
		httputil.RespondInternalError(c, "Internal server error retrieving session", err, "AUTH_ME_FAILED")
		return
	}

	c.JSON(http.StatusOK, user)
}
