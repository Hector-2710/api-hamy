package dto

// ListUsersQuery holds the query params of GET /api/v1/admin/users.
type ListUsersQuery struct {
	Page    int    `form:"page,default=1" binding:"min=1"`
	PerPage int    `form:"per_page,default=20" binding:"min=1,max=100"`
	Search  string `form:"search"`
}

// UpdateAdminUserRequest is the body of PATCH /api/v1/admin/users/:id.
// All fields are optional.
type UpdateAdminUserRequest struct {
	Name    *string `json:"name" binding:"omitempty,min=2,max=100"`
	Email   *string `json:"email" binding:"omitempty,email"`
	IsAdmin *bool   `json:"is_admin"`
}
