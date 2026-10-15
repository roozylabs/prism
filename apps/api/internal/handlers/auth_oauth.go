package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/service"
)

// oauthStateCookie carries the state a sign-in started with, so the callback
// can tell its own redirect from a forged one.
const (
	oauthStateCookie = "oauth_state"
	oauthStatePath   = "/api/auth"
	oauthStateTTL    = 10 * time.Minute
)

var errUnverifiedEmail = errors.New("provider returned no verified email")

type oauthSessionCreator interface {
	CreateOAuthSession(ctx context.Context, email, name, image, provider, ip, ua string) (*service.LoginResponse, error)
}

// oauthEndpoints are the provider URLs sign-in calls; tests point them at a
// local server.
type oauthEndpoints struct {
	googleAuth, googleToken, googleUserInfo          string
	githubAuth, githubToken, githubUser, githubEmail string
}

var defaultOAuthEndpoints = oauthEndpoints{
	googleAuth:     "https://accounts.google.com/o/oauth2/v2/auth",
	googleToken:    "https://oauth2.googleapis.com/token",
	googleUserInfo: "https://www.googleapis.com/oauth2/v2/userinfo",
	githubAuth:     "https://github.com/login/oauth/authorize",
	githubToken:    "https://github.com/login/oauth/access_token",
	githubUser:     "https://api.github.com/user",
	githubEmail:    "https://api.github.com/user/emails",
}

type OAuthHandler struct {
	authService oauthSessionCreator
	endpoints   oauthEndpoints
	client      *http.Client
	// allowDevLogin signs in a fixed developer account when a provider has no
	// client ID. Only APP_ENV=development, set explicitly, enables it.
	allowDevLogin bool
}

func NewOAuthHandler(authService *service.AuthService) *OAuthHandler {
	return &OAuthHandler{
		authService:   authService,
		endpoints:     defaultOAuthEndpoints,
		client:        &http.Client{Timeout: 15 * time.Second},
		allowDevLogin: strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "development"),
	}
}

func (h *OAuthHandler) getBaseURL(c *gin.Context) string {
	baseURL := os.Getenv("BETTER_AUTH_URL")
	if baseURL == "" {
		baseURL = os.Getenv("PUBLIC_URL")
	}
	if baseURL == "" && c != nil {
		scheme := "https"
		if c.Request.TLS == nil && c.GetHeader("X-Forwarded-Proto") != "https" {
			if strings.HasPrefix(c.Request.Host, "localhost") || strings.HasPrefix(c.Request.Host, "127.0.0.1") {
				scheme = "http"
			}
		}
		host := c.GetHeader("X-Forwarded-Host")
		if host == "" {
			host = c.Request.Host
		}
		if host != "" {
			baseURL = fmt.Sprintf("%s://%s", scheme, host)
		}
	}
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}
	return strings.TrimRight(baseURL, "/")
}

func (h *OAuthHandler) GoogleLogin(c *gin.Context) {
	c.Params = []gin.Param{{Key: "provider", Value: "google"}}
	h.InitiateOAuth(c)
}

func (h *OAuthHandler) GoogleCallback(c *gin.Context) {
	c.Params = []gin.Param{{Key: "provider", Value: "google"}}
	h.OAuthCallback(c)
}

func (h *OAuthHandler) GitHubLogin(c *gin.Context) {
	c.Params = []gin.Param{{Key: "provider", Value: "github"}}
	h.InitiateOAuth(c)
}

func (h *OAuthHandler) GitHubCallback(c *gin.Context) {
	c.Params = []gin.Param{{Key: "provider", Value: "github"}}
	h.OAuthCallback(c)
}

func (h *OAuthHandler) InitiateOAuth(c *gin.Context) {
	provider := strings.ToLower(c.Param("provider"))
	if provider != "google" && provider != "github" {
		httputil.RespondBadRequest(c, "Unsupported OAuth provider", nil, "UNSUPPORTED_OAUTH_PROVIDER")
		return
	}

	clientID, _ := oauthClient(provider)
	if clientID == "" {
		if h.allowDevLogin {
			h.devLogin(c, provider)
			return
		}
		c.Redirect(http.StatusTemporaryRedirect, "/signin?error=oauth_not_configured")
		return
	}

	state, err := newOAuthState()
	if err != nil {
		httputil.RespondInternalError(c, "Failed to start OAuth sign-in", err, "OAUTH_STATE_FAILED")
		return
	}
	setOAuthCookie(c, oauthStateCookie, state, int(oauthStateTTL.Seconds()), oauthStatePath)

	redirectURI := fmt.Sprintf("%s/api/auth/%s/callback", h.getBaseURL(c), provider)
	var authURL string
	switch provider {
	case "google":
		authURL = fmt.Sprintf(
			"%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&access_type=offline&prompt=consent&state=%s",
			h.endpoints.googleAuth,
			url.QueryEscape(clientID),
			url.QueryEscape(redirectURI),
			strings.ReplaceAll(url.QueryEscape("openid email profile"), "+", "%20"),
			url.QueryEscape(state),
		)
	case "github":
		authURL = fmt.Sprintf(
			"%s?client_id=%s&redirect_uri=%s&scope=%s&state=%s",
			h.endpoints.githubAuth,
			url.QueryEscape(clientID),
			url.QueryEscape(redirectURI),
			url.QueryEscape("read:user user:email"),
			url.QueryEscape(state),
		)
	}
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// devLogin signs in a fixed local developer account without any provider.
func (h *OAuthHandler) devLogin(c *gin.Context, provider string) {
	email := fmt.Sprintf("%s.developer@roozylabs.dev", provider)
	name := fmt.Sprintf("%s Developer", strings.ToUpper(provider[:1])+provider[1:])
	resp, err := h.authService.CreateOAuthSession(c.Request.Context(), email, name, "", provider, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		httputil.RespondInternalError(c, "Failed to create OAuth session", err, "OAUTH_SESSION_CREATION_FAILED")
		return
	}
	setOAuthCookie(c, "auth_token", resp.Token, 7*24*3600, "/")
	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func (h *OAuthHandler) OAuthCallback(c *gin.Context) {
	provider := strings.ToLower(c.Param("provider"))
	if provider != "google" && provider != "github" {
		c.Redirect(http.StatusTemporaryRedirect, "/signin?error=unsupported_provider")
		return
	}

	// The state check comes first: without it, anyone could finish a sign-in
	// they did not start.
	if !consumeOAuthState(c) {
		c.Redirect(http.StatusTemporaryRedirect, "/signin?error=invalid_state")
		return
	}
	if c.Query("error") != "" {
		c.Redirect(http.StatusTemporaryRedirect, "/signin?error=oauth_denied")
		return
	}
	code := c.Query("code")
	if code == "" {
		c.Redirect(http.StatusTemporaryRedirect, "/signin?error=missing_code")
		return
	}

	redirectURI := fmt.Sprintf("%s/api/auth/%s/callback", h.getBaseURL(c), provider)
	var (
		id  oauthIdentity
		err error
	)
	switch provider {
	case "google":
		id, err = h.googleIdentity(c.Request.Context(), code, redirectURI)
	case "github":
		id, err = h.githubIdentity(c.Request.Context(), code, redirectURI)
	}
	if err != nil {
		log.Printf("[OAuth Callback] provider=%s: %v", provider, err)
		reason := "oauth_failed"
		if errors.Is(err, errUnverifiedEmail) {
			reason = "email_unverified"
		}
		c.Redirect(http.StatusTemporaryRedirect, "/signin?error="+reason)
		return
	}

	resp, err := h.authService.CreateOAuthSession(c.Request.Context(), id.email, id.name, id.avatar, provider, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		log.Printf("[OAuth Callback Session Error] provider=%s email=%s err=%v", provider, id.email, err)
		c.Redirect(http.StatusTemporaryRedirect, "/signin?error=session_creation_failed")
		return
	}

	setOAuthCookie(c, "auth_token", resp.Token, 7*24*3600, "/")
	c.Redirect(http.StatusTemporaryRedirect, "/")
}

type oauthIdentity struct {
	email, name, avatar string
}

func (h *OAuthHandler) googleIdentity(ctx context.Context, code, redirectURI string) (oauthIdentity, error) {
	clientID, clientSecret := oauthClient("google")
	token, err := h.exchangeCode(ctx, h.endpoints.googleToken, url.Values{
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		return oauthIdentity{}, err
	}

	var info struct {
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := h.getJSON(ctx, h.endpoints.googleUserInfo, token, &info); err != nil {
		return oauthIdentity{}, err
	}
	email := strings.TrimSpace(info.Email)
	if email == "" || !info.VerifiedEmail {
		return oauthIdentity{}, errUnverifiedEmail
	}
	return oauthIdentity{email: email, name: strings.TrimSpace(info.Name), avatar: strings.TrimSpace(info.Picture)}, nil
}

func (h *OAuthHandler) githubIdentity(ctx context.Context, code, redirectURI string) (oauthIdentity, error) {
	clientID, clientSecret := oauthClient("github")
	token, err := h.exchangeCode(ctx, h.endpoints.githubToken, url.Values{
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
	})
	if err != nil {
		return oauthIdentity{}, err
	}

	var user struct {
		Name      string `json:"name"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := h.getJSON(ctx, h.endpoints.githubUser, token, &user); err != nil {
		return oauthIdentity{}, err
	}

	// The profile email is optional and unverified there, so the account is
	// matched only on a verified address from /user/emails.
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := h.getJSON(ctx, h.endpoints.githubEmail, token, &emails); err != nil {
		return oauthIdentity{}, err
	}
	email := ""
	for _, e := range emails {
		if e.Verified && e.Primary {
			email = e.Email
			break
		}
		if e.Verified && email == "" {
			email = e.Email
		}
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return oauthIdentity{}, errUnverifiedEmail
	}

	name := strings.TrimSpace(user.Name)
	if name == "" {
		name = strings.TrimSpace(user.Login)
	}
	return oauthIdentity{email: email, name: name, avatar: strings.TrimSpace(user.AvatarURL)}, nil
}

// exchangeCode trades an authorization code for an access token. GitHub
// reports a bad code with 200 and an "error" field, so a missing token is a
// failure whatever the status.
func (h *OAuthHandler) exchangeCode(ctx context.Context, tokenURL string, form url.Values) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = json.Unmarshal(body, &tok)
	if resp.StatusCode != http.StatusOK || tok.AccessToken == "" {
		return "", fmt.Errorf("token exchange: status %d, error %q", resp.StatusCode, tok.Error)
	}
	return tok.AccessToken, nil
}

func (h *OAuthHandler) getJSON(ctx context.Context, endpoint, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", endpoint, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
		return fmt.Errorf("GET %s: %w", endpoint, err)
	}
	return nil
}

func oauthClient(provider string) (clientID, clientSecret string) {
	switch provider {
	case "google":
		return os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET")
	case "github":
		return os.Getenv("GITHUB_CLIENT_ID"), os.Getenv("GITHUB_CLIENT_SECRET")
	}
	return "", ""
}

func newOAuthState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// consumeOAuthState checks the callback's state against the cookie set when
// the sign-in started, and clears the cookie so a state works only once.
func consumeOAuthState(c *gin.Context) bool {
	cookie, err := c.Cookie(oauthStateCookie)
	setOAuthCookie(c, oauthStateCookie, "", -1, oauthStatePath)
	state := c.Query("state")
	return err == nil && cookie != "" && state != "" &&
		subtle.ConstantTimeCompare([]byte(cookie), []byte(state)) == 1
}

// setOAuthCookie sets an HttpOnly, SameSite=Lax cookie. Lax still sends it on
// the provider's top-level redirect back to the callback.
func setOAuthCookie(c *gin.Context, name, value string, maxAge int, path string) {
	c.SetSameSite(http.SameSiteLaxMode)
	isSecure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetCookie(name, value, maxAge, path, "", isSecure, true)
}
