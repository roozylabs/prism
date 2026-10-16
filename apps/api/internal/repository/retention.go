package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// RetentionRepository deletes old rows in small batches, so a large purge
// never holds long locks or one huge transaction.
type RetentionRepository struct {
	db *sql.DB
}

func NewRetentionRepository(db *sql.DB) *RetentionRepository {
	return &RetentionRepository{db: db}
}

// PurgeBefore deletes rows of table whose column is older than cutoff,
// batchSize rows per statement, pausing between batches. It stops after
// maxBatches so one run stays bounded; the next run continues. table and
// column come from code, never from a request.
func (r *RetentionRepository) PurgeBefore(ctx context.Context, table, column string, cutoff time.Time, batchSize, maxBatches int, pause time.Duration) (int64, error) {
	t, c := pq.QuoteIdentifier(table), pq.QuoteIdentifier(column)
	query := fmt.Sprintf(`DELETE FROM %s WHERE ctid = ANY(ARRAY(SELECT ctid FROM %s WHERE %s < $1 LIMIT $2))`, t, t, c)
	var total int64
	for i := 0; i < maxBatches; i++ {
		res, err := r.db.ExecContext(ctx, query, cutoff, batchSize)
		if err != nil {
			return total, fmt.Errorf("purge %s: %w", table, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, err
		}
		total += n
		if n < int64(batchSize) {
			return total, nil
		}
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		case <-time.After(pause):
		}
	}
	return total, nil
}
