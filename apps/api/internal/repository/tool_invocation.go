package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/roozylabs/prism/internal/models"
)

type ToolInvocationRepository struct {
	db *sql.DB
}

func NewToolInvocationRepository(db *sql.DB) *ToolInvocationRepository {
	return &ToolInvocationRepository{db: db}
}

func (r *ToolInvocationRepository) CreateBatch(ctx context.Context, requestID string, records []models.ToolCallRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt := &strings.Builder{}
	stmt.WriteString("INSERT INTO tool_invocations (request_id, tool_name, call_id, arguments) VALUES ")

	args := []interface{}{}
	for i, rec := range records {
		if i > 0 {
			stmt.WriteString(", ")
		}
		offset := i * 4
		_, _ = fmt.Fprintf(stmt, "($%d, $%d, $%d, $%d::jsonb)", offset+1, offset+2, offset+3, offset+4)

		var callID interface{}
		if rec.CallID != "" {
			callID = rec.CallID
		}
		var arguments interface{}
		if rec.Arguments != nil {
			arguments = []byte(rec.Arguments)
		}
		args = append(args, requestID, rec.Name, callID, arguments)
	}

	if _, err := tx.ExecContext(ctx, stmt.String(), args...); err != nil {
		return fmt.Errorf("insert tool invocations: %w", err)
	}

	return tx.Commit()
}
