package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
)

type GoogleOAuthHandler struct {
	credentials *repository.CredentialRepository
	providers   *repository.ProviderRepository
	encKey      string
}

func NewGoogleOAuthHandler(credentials *repository.CredentialRepository, providers *repository.ProviderRepository, encKey string) *GoogleOAuthHandler {
	return &GoogleOAuthHandler{
		credentials: credentials,
		providers:   providers,
		encKey:      encKey,
	}
}

func (h *GoogleOAuthHandler) getOAuthEnv(c *gin.Context) (clientID, clientSecret, redirectURI string) {
	clientID = os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
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
	redirectURI = strings.TrimRight(baseURL, "/") + "/api/auth/google/callback"
	return clientID, clientSecret, redirectURI
}

func (h *GoogleOAuthHandler) Login(c *gin.Context) {
	providerID := c.Query("provider_id")
	if providerID == "" {
		c.Redirect(http.StatusTemporaryRedirect, "/api/auth/oauth/google")
		return
	}

	clientID, _, redirectURI := h.getOAuthEnv(c)
	if clientID == "" {
		httputil.RespondInternalError(c, "GOOGLE_CLIENT_ID is not configured in environment", nil, "GOOGLE_CLIENT_ID_MISSING")
		return
	}

	state := providerID
	scopes := "https://www.googleapis.com/auth/generative-language openid email profile"

	authURL := fmt.Sprintf(
		"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&access_type=offline&prompt=consent&state=%s",
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		strings.ReplaceAll(url.QueryEscape(scopes), "+", "%20"),
		url.QueryEscape(state),
	)

	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

func (h *GoogleOAuthHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	providerID := c.Query("state")
	errQuery := c.Query("error")

	if errQuery != "" {
		renderPopupResult(c, false, "", "Google OAuth Error: "+errQuery)
		return
	}

	if code == "" || providerID == "" {
		renderPopupResult(c, false, "", "Invalid OAuth callback code or state")
		return
	}

	clientID, clientSecret, redirectURI := h.getOAuthEnv(c)

	// 1. Exchange code for tokens
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")

	resp, err := http.PostForm("https://oauth2.googleapis.com/token", form)
	if err != nil {
		renderPopupResult(c, false, "", "Token exchange request failed: "+err.Error())
		return
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		renderPopupResult(c, false, "", "Token exchange failed: "+string(bodyBytes))
		return
	}

	var tokenData struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(bodyBytes, &tokenData); err != nil {
		renderPopupResult(c, false, "", "Failed to parse token response")
		return
	}

	// 2. Fetch User Profile Email from Google
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
	userResp, err := http.DefaultClient.Do(req)
	var userEmail string
	if err == nil && userResp.StatusCode == http.StatusOK {
		var userInfo struct {
			Email string `json:"email"`
		}
		_ = json.NewDecoder(userResp.Body).Decode(&userInfo)
		userEmail = userInfo.Email
		_ = userResp.Body.Close()
	}

	credName := "Gemini Account"
	if userEmail != "" {
		credName = fmt.Sprintf("Gemini (%s)", userEmail)
	}

	// 3. Encrypt OAuth metadata
	metaMap := map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"refresh_token": tokenData.RefreshToken,
	}
	metaBytes, _ := json.Marshal(metaMap)
	encMeta, err := utils.EncryptAES256GCM(string(metaBytes), h.encKey)
	if err != nil {
		renderPopupResult(c, false, "", "Failed to encrypt OAuth metadata")
		return
	}

	encKeyPlaceholder, _ := utils.EncryptAES256GCM("oauth_token", h.encKey)

	keyPrefix := clientID
	if len(keyPrefix) > 12 {
		keyPrefix = keyPrefix[:12]
	}
	maskedKey := userEmail
	if maskedKey == "" {
		maskedKey = "gcp_oauth_key"
	}

	// 4. Save Credential
	cred := &models.Credential{
		ProviderID:        providerID,
		Name:              credName,
		EncryptedKey:      encKeyPlaceholder,
		KeyPrefix:         keyPrefix,
		MaskedKey:         maskedKey,
		AuthType:          models.AuthTypeGCPUserOAuth,
		EncryptedMetadata: sql.NullString{String: encMeta, Valid: true},
		Priority:          1,
		Enabled:           true,
		Status:            models.CredentialStatusActive,
	}

	if err := h.credentials.Create(c.Request.Context(), cred); err != nil {
		renderPopupResult(c, false, "", "Failed to save credential to database: "+err.Error())
		return
	}

	renderPopupResult(c, true, userEmail, "")
}

// renderPopupResult reports the result to the opener window. email and errMsg
// can carry query input (the provider's error parameter), so they are escaped
// for HTML and passed to the script as JSON, which also escapes '<' and '>'.
func renderPopupResult(c *gin.Context, success bool, email, errMsg string) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	if success {
		emailJS, _ := json.Marshal(email)
		page := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Google OAuth Success</title></head>
<body style="font-family: system-ui; background: #141414; color: #fff; text-align: center; padding-top: 40px;">
  <h2>✅ Google Account Connected!</h2>
  <p>Account: %s</p>
  <p>Closing window...</p>
  <script>
    if (window.opener) {
      window.opener.postMessage({ type: 'google_oauth_success', email: %s }, '*');
    }
    setTimeout(() => window.close(), 1200);
  </script>
</body>
</html>`, html.EscapeString(email), emailJS)
		c.String(http.StatusOK, page)
	} else {
		errJS, _ := json.Marshal(errMsg)
		page := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Google OAuth Failed</title></head>
<body style="font-family: system-ui; background: #141414; color: #ff4d4f; text-align: center; padding-top: 40px;">
  <h2>❌ Authentication Failed</h2>
  <p>%s</p>
  <script>
    if (window.opener) {
      window.opener.postMessage({ type: 'google_oauth_error', error: %s }, '*');
    }
  </script>
</body>
</html>`, html.EscapeString(errMsg), errJS)
		c.String(http.StatusBadRequest, page)
	}
}
