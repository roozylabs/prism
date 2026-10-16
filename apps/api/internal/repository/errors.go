package repository

import (
	"errors"

	"github.com/lib/pq"
)

// IsUniqueViolation reports whether err is a unique-constraint violation
// (SQLSTATE 23505), which handlers answer with 409 Conflict.
func IsUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
