package dto

// CreateUserRequest is the body of POST /api/v1/users.
type CreateUserRequest struct {
	Name  string `json:"name" binding:"required,min=2,max=100"`
	Email string `json:"email" binding:"required,email"`
}

// UpdateUserRequest is the body of PATCH /api/v1/users/:id (all fields optional).
type UpdateUserRequest struct {
	Name  *string `json:"name" binding:"omitempty,min=2,max=100"`
	Email *string `json:"email" binding:"omitempty,email"`
}

// ListUsersQuery holds the query params of GET /api/v1/users.
type ListUsersQuery struct {
	Page    int    `form:"page,default=1" binding:"min=1"`
	PerPage int    `form:"per_page,default=20" binding:"min=1,max=100"`
	Search  string `form:"search"`
}
