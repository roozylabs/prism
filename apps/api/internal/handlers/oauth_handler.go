package handlers

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/utils"
)

// Connecting a Google account as a Gemini credential. Both routes require a
// signed-in user; the state cookie ties the callback to the browser and the
// provider that started it.
const (
	credentialOAuthStateCookie = "gcp_oauth_state"
	credentialOAuthStatePath   = "/api/providers/google/oauth"
	credentialOAuthCallback    = "/api/providers/google/oauth/callback"
)

type credentialProviderFinder interface {
	FindByID(ctx context.Context, id string) (*models.Provider, error)
}

type credentialCreator interface {
	Create(ctx context.Context, c *models.Credential) error
}

type GoogleOAuthHandler struct {
	credentials credentialCreator
	providers   credentialProviderFinder
	encKey      string
	client      *http.Client
	// tokenURL and userInfoURL are Google's endpoints; tests replace them.
	tokenURL    string
	userInfoURL string
	authURL     string
}

func NewGoogleOAuthHandler(credentials credentialCreator, providers credentialProviderFinder, encKey string) *GoogleOAuthHandler {
	return &GoogleOAuthHandler{
		credentials: credentials,
		providers:   providers,
		encKey:      encKey,
		client:      &http.Client{Timeout: 15 * time.Second},
		tokenURL:    defaultOAuthEndpoints.googleToken,
		userInfoURL: defaultOAuthEndpoints.googleUserInfo,
		authURL:     defaultOAuthEndpoints.googleAuth,
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
	// This flow has its own callback. The sign-in callback would treat the
	// code as a sign-in and never store the credential.
	redirectURI = strings.TrimRight(baseURL, "/") + credentialOAuthCallback
	return clientID, clientSecret, redirectURI
}

// googleProviderDenial says why the signed-in user may not attach a Gemini
// credential to providerID, in words the popup can show, or "" if they may.
func (h *GoogleOAuthHandler) googleProviderDenial(ctx context.Context, providerID, userID string) string {
	if userID == "" {
		return "Sign in to connect a Google account."
	}
	prov, err := h.providers.FindByID(ctx, providerID)
	if err != nil || !providerAcceptsCredentialsFrom(prov, userID) {
		return "You cannot add credentials to this provider."
	}
	if prov.Type != "google" {
		return "Google accounts can only be connected to a Google provider."
	}
	return ""
}

func (h *GoogleOAuthHandler) Login(c *gin.Context) {
	providerID := c.Query("provider_id")
	if providerID == "" {
		renderPopupResult(c, false, "", "provider_id is required")
		return
	}
	if denial := h.googleProviderDenial(c.Request.Context(), providerID, c.GetString("userId")); denial != "" {
		renderPopupResult(c, false, "", denial)
		return
	}

	clientID, _, redirectURI := h.getOAuthEnv(c)
	if clientID == "" {
		renderPopupResult(c, false, "", "Google sign-in is not configured on this server")
		return
	}

	nonce, err := newOAuthState()
	if err != nil {
		renderPopupResult(c, false, "", "Could not start the Google sign-in")
		return
	}
	// The provider rides in the state, which the callback trusts only after
	// matching it against this cookie.
	state := nonce + "." + providerID
	setOAuthCookie(c, credentialOAuthStateCookie, state, int(oauthStateTTL.Seconds()), credentialOAuthStatePath)

	scopes := "https://www.googleapis.com/auth/generative-language openid email profile"
	authURL := fmt.Sprintf(
		"%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&access_type=offline&prompt=consent&state=%s",
		h.authURL,
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		strings.ReplaceAll(url.QueryEscape(scopes), "+", "%20"),
		url.QueryEscape(state),
	)
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

func (h *GoogleOAuthHandler) Callback(c *gin.Context) {
	cookie, cookieErr := c.Cookie(credentialOAuthStateCookie)
	setOAuthCookie(c, credentialOAuthStateCookie, "", -1, credentialOAuthStatePath)
	state := c.Query("state")
	if cookieErr != nil || cookie == "" || state == "" || subtle.ConstantTimeCompare([]byte(cookie), []byte(state)) != 1 {
		renderPopupResult(c, false, "", "This connection was not started here or has expired. Start it again from Credentials.")
		return
	}
	_, providerID, _ := strings.Cut(state, ".")

	if errQuery := c.Query("error"); errQuery != "" {
		renderPopupResult(c, false, "", "Google OAuth Error: "+errQuery)
		return
	}
	code := c.Query("code")
	if code == "" || providerID == "" {
		renderPopupResult(c, false, "", "Invalid OAuth callback code or state")
		return
	}

	// Check again: access may have changed while the user was at Google.
	userID := c.GetString("userId")
	if denial := h.googleProviderDenial(c.Request.Context(), providerID, userID); denial != "" {
		renderPopupResult(c, false, "", denial)
		return
	}

	clientID, clientSecret, redirectURI := h.getOAuthEnv(c)
	tokenData, err := h.exchangeCode(c.Request.Context(), url.Values{
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		log.Printf("[Google credential OAuth] token exchange: %v", err)
		renderPopupResult(c, false, "", "Google did not accept the authorization. Try connecting again.")
		return
	}
	if tokenData.RefreshToken == "" {
		renderPopupResult(c, false, "", "Google returned no refresh token, so the credential could not be saved. Try connecting again.")
		return
	}

	userEmail := h.fetchEmail(c.Request.Context(), tokenData.AccessToken)
	credName := "Gemini Account"
	if userEmail != "" {
		credName = fmt.Sprintf("Gemini (%s)", userEmail)
	}

	metaBytes, _ := json.Marshal(map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"refresh_token": tokenData.RefreshToken,
	})
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

	cred := &models.Credential{
		ProviderID:        providerID,
		UserID:            &userID,
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
	if orgID := c.GetString("organization_id"); orgID != "" {
		cred.OrgID = &orgID
	}

	if err := h.credentials.Create(c.Request.Context(), cred); err != nil {
		log.Printf("[Google credential OAuth] save credential: %v", err)
		renderPopupResult(c, false, "", "Failed to save the credential")
		return
	}

	renderPopupResult(c, true, userEmail, "")
}

type googleTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (h *GoogleOAuthHandler) exchangeCode(ctx context.Context, form url.Values) (googleTokens, error) {
	var tok googleTokens
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tok, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return tok, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return tok, fmt.Errorf("status %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return tok, fmt.Errorf("no access token in response")
	}
	return tok, nil
}

// fetchEmail names the credential after the Google account; it is optional.
func (h *GoogleOAuthHandler) fetchEmail(ctx context.Context, accessToken string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.userInfoURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := h.client.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var info struct {
		Email string `json:"email"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&info)
	return info.Email
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
