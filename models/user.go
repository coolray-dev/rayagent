package models

// User model
type User struct {
	Email          string `json:"email" validate:"required,email"`
	Username       string `json:"username" validate:"required"`
	CurrentTraffic uint64 `json:"current_traffic"`
	MaxTraffic     uint64 `json:"max_traffic" validate:"required"`
}
