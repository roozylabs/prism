package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailAlreadyExists = errors.New("email already registered")
	ErrSessionExpired     = errors.New("session expired")
	ErrSessionNotFound    = errors.New("session not found")
)

// InvalidSignupError explains which sign-up field was rejected.
type InvalidSignupError struct{ Reason string }

func (e *InvalidSignupError) Error() string { return e.Reason }

const (
	minPasswordLen = 8
	// bcrypt ignores everything after 72 bytes, so a longer password would
	// silently match any password sharing its first 72 bytes.
	maxPasswordBytes = 72
	maxNameLen       = 100
)

// validateSignup checks the fields of a new account. email and name are
// already trimmed.
func validateSignup(name, email, password string) error {
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@")+1:], ".") {
		return &InvalidSignupError{Reason: "Enter a valid email address"}
	}
	if name == "" || utf8.RuneCountInString(name) > maxNameLen {
		return &InvalidSignupError{Reason: fmt.Sprintf("Name must be between 1 and %d characters", maxNameLen)}
	}
	if utf8.RuneCountInString(password) < minPasswordLen {
		return &InvalidSignupError{Reason: fmt.Sprintf("Password must be at least %d characters", minPasswordLen)}
	}
	if len(password) > maxPasswordBytes {
		return &InvalidSignupError{Reason: fmt.Sprintf("Password must be at most %d bytes", maxPasswordBytes)}
	}
	return nil
}

type AuthService struct {
	users    *repository.UserRepository
	sessions *repository.SessionRepository
	accounts *repository.AccountRepository
}

func NewAuthService(
	users *repository.UserRepository,
	sessions *repository.SessionRepository,
	accounts *repository.AccountRepository,
) *AuthService {
	return &AuthService{
		users:    users,
		sessions: sessions,
		accounts: accounts,
	}
}

type LoginRequest struct {
	Email          string `json:"email" binding:"required"`
	Password       string `json:"password" binding:"required"`
	TurnstileToken string `json:"turnstileToken"`
}

type SignupRequest struct {
	Name           string `json:"name" binding:"required"`
	Email          string `json:"email" binding:"required"`
	Password       string `json:"password" binding:"required"`
	TurnstileToken string `json:"turnstileToken"`
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

func (s *AuthService) Login(ctx context.Context, email, password, ip, ua string) (*LoginResponse, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	account, err := s.accounts.FindByUserID(ctx, user.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !utils.CheckPassword(password, account.Password) {
		return nil, ErrInvalidCredentials
	}

	token := utils.HashSHA256(uuid.New().String())
	session := &models.Session{
		ID:        uuid.New().String(),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Token:     token,
		IPAddress: ip,
		UserAgent: ua,
		UserID:    user.ID,
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, err
	}

	return &LoginResponse{Token: token, User: user}, nil
}

func (s *AuthService) Signup(ctx context.Context, name, email, password, ip, ua string) (*LoginResponse, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)
	if err := validateSignup(name, email, password); err != nil {
		return nil, err
	}

	// Check if user already exists
	existing, err := s.users.FindByEmail(ctx, email)
	if err == nil && existing != nil {
		return nil, ErrEmailAlreadyExists
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}

	userID := uuid.New().String()
	user := &models.User{
		ID:            userID,
		Name:          name,
		Email:         email,
		EmailVerified: false,
		IsOnboarded:   false,
		AuthProvider:  "credential",
		PrimaryRole:   "owner",
	}

	if err := s.users.Create(ctx, user); err != nil {
		log.Printf("[AuthService Signup] failed to create user %s: %v", email, err)
		return nil, err
	}

	account := &models.Account{
		ID:         uuid.New().String(),
		AccountID:  userID,
		ProviderID: "credential",
		UserID:     userID,
		Password:   hashedPassword,
	}

	if err := s.accounts.Create(ctx, account); err != nil {
		log.Printf("[AuthService Signup] failed to create account %s: %v", email, err)
		return nil, err
	}

	token := utils.HashSHA256(uuid.New().String())
	session := &models.Session{
		ID:        uuid.New().String(),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Token:     token,
		IPAddress: ip,
		UserAgent: ua,
		UserID:    user.ID,
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		log.Printf("[AuthService Signup] failed to create session %s: %v", email, err)
		return nil, err
	}

	return &LoginResponse{Token: token, User: user}, nil
}

func (s *AuthService) CreateOAuthSession(ctx context.Context, email, name, image, provider, ip, ua string) (*LoginResponse, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		// Without a verified email there is no account to match; falling back
		// to a placeholder would put every such sign-in on one shared account.
		return nil, errors.New("oauth sign-in requires a verified email")
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			userID := uuid.New().String()
			userName := strings.TrimSpace(name)
			if userName == "" {
				userName = strings.ToUpper(provider[:1]) + provider[1:] + " Developer"
			}
			user = &models.User{
				ID:            userID,
				Name:          userName,
				Email:         email,
				EmailVerified: true,
				Image:         image,
				AvatarURL:     image,
				IsOnboarded:   false,
				AuthProvider:  provider,
				PrimaryRole:   "developer",
			}
			if createErr := s.users.Create(ctx, user); createErr != nil {
				log.Printf("[AuthService CreateOAuthSession] failed to create user %s: %v", email, createErr)
				return nil, createErr
			}
		} else {
			log.Printf("[AuthService CreateOAuthSession] find by email error %s: %v", email, err)
			return nil, err
		}
	} else if user != nil {
		// Update user profile info and ensure email_verified is true
		if strings.TrimSpace(name) != "" && user.Name == "" {
			user.Name = strings.TrimSpace(name)
		}
		if strings.TrimSpace(image) != "" && user.Image == "" {
			user.Image = strings.TrimSpace(image)
			user.AvatarURL = strings.TrimSpace(image)
		}
		user.EmailVerified = true
		_ = s.users.UpdateProfile(ctx, user)
	}

	// Create account record if it does not exist
	account := &models.Account{
		ID:         uuid.New().String(),
		AccountID:  user.ID,
		ProviderID: provider,
		UserID:     user.ID,
		Password:   "",
	}
	_ = s.accounts.Create(ctx, account)

	token := utils.HashSHA256(uuid.New().String())
	session := &models.Session{
		ID:        uuid.New().String(),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Token:     token,
		IPAddress: ip,
		UserAgent: ua,
		UserID:    user.ID,
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		log.Printf("[AuthService CreateOAuthSession] failed to create session for %s: %v", email, err)
		return nil, err
	}

	return &LoginResponse{Token: token, User: user}, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	return s.sessions.Delete(ctx, token)
}

func (s *AuthService) Me(ctx context.Context, token string) (*models.User, error) {
	session, err := s.sessions.FindValidByToken(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	if session.ExpiresAt.Before(time.Now()) {
		return nil, ErrSessionExpired
	}

	user, err := s.users.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	return user, nil
}
