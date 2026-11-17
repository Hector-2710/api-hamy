package postgres

import (
	"context"

	"api-hamy/internal/domain"
	"api-hamy/internal/repository/postgres/sqlcgen"
)

// UserRepository implements domain.UserRepository on Postgres.
type UserRepository struct {
	q *sqlcgen.Queries
}

// Compile-time check that the domain contract is satisfied.
var _ domain.UserRepository = (*UserRepository)(nil)

func NewUserRepository(db sqlcgen.DBTX) *UserRepository {
	return &UserRepository{q: sqlcgen.New(db)}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	row, err := r.q.CreateUser(ctx, sqlcgen.CreateUserParams{
		Name:  u.Name,
		Email: u.Email,
	})
	if err != nil {
		return mapErr(err)
	}

	*u = mapUser(row)
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}

	u := mapUser(row)
	return &u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, mapErr(err)
	}

	u := mapUser(row)
	return &u, nil
}

func (r *UserRepository) List(ctx context.Context, f domain.ListFilter) ([]domain.User, int64, error) {
	total, err := r.q.CountUsers(ctx, f.Search)
	if err != nil {
		return nil, 0, mapErr(err)
	}

	rows, err := r.q.ListUsers(ctx, sqlcgen.ListUsersParams{
		Search: f.Search,
		Off:    int32(f.Offset()),
		Lim:    int32(f.PerPage),
	})
	if err != nil {
		return nil, 0, mapErr(err)
	}

	users := make([]domain.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, mapUser(row))
	}

	return users, total, nil
}

func (r *UserRepository) Update(ctx context.Context, u *domain.User) error {
	row, err := r.q.UpdateUser(ctx, sqlcgen.UpdateUserParams{
		Name:  &u.Name,
		Email: &u.Email,
		ID:    u.ID,
	})
	if err != nil {
		return mapErr(err)
	}

	*u = mapUser(row)
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	rows, err := r.q.SoftDeleteUser(ctx, id)
	if err != nil {
		return mapErr(err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}
