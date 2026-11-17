package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"api-hamy/internal/domain"
)

// mapErr translates Postgres errors into domain errors.
// The rest of the application never sees pgx or SQLSTATE codes.
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, pgx.ErrNoRows):
		return domain.ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return domain.ErrConflict
		case "23503": // foreign_key_violation
			return domain.ErrInvalid
		case "22P02": // invalid_text_representation (malformed uuid)
			return domain.ErrInvalid
		}
	}

	return fmt.Errorf("postgres: %w", err)
}
