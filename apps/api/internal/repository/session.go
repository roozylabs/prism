package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, session *models.Session) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	session.CreatedAt = time.Now()
	session.UpdatedAt = time.Now()

	var ipAddress, userAgent sql.NullString
	if strings.TrimSpace(session.IPAddress) != "" {
		ipAddress = sql.NullString{String: strings.TrimSpace(session.IPAddress), Valid: true}
	}
	if strings.TrimSpace(session.UserAgent) != "" {
		userAgent = sql.NullString{String: strings.TrimSpace(session.UserAgent), Valid: true}
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO session (id, expires_at, token, ip_address, user_agent, user_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		session.ID, session.ExpiresAt, session.Token,
		ipAddress, userAgent, session.UserID,
		session.CreatedAt, session.UpdatedAt,
	)
	return err
}

func (r *SessionRepository) FindByToken(ctx context.Context, token string) (*models.Session, error) {
	session := &models.Session{}
	var ipAddress, userAgent sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, expires_at, token, ip_address, user_agent, user_id, created_at, updated_at
		 FROM session WHERE token = $1`, token,
	).Scan(&session.ID, &session.ExpiresAt, &session.Token,
		&ipAddress, &userAgent, &session.UserID,
		&session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return nil, err
	}
	session.IPAddress = ipAddress.String
	session.UserAgent = userAgent.String
	return session, nil
}

func (r *SessionRepository) FindValidByToken(ctx context.Context, token string) (*models.Session, error) {
	session := &models.Session{}
	var ipAddress, userAgent sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, expires_at, token, ip_address, user_agent, user_id, created_at, updated_at
		 FROM session WHERE token = $1 AND expires_at > NOW()`, token,
	).Scan(&session.ID, &session.ExpiresAt, &session.Token,
		&ipAddress, &userAgent, &session.UserID,
		&session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return nil, err
	}
	session.IPAddress = ipAddress.String
	session.UserAgent = userAgent.String
	return session, nil
}

func (r *SessionRepository) Delete(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM session WHERE token = $1`, token)
	return err
}
