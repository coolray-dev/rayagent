package models

// User model
type User struct {
	Email          string `json:"email"`
	Username       string `json:"username"`
	CurrentTraffic uint64 `json:"current_traffic"`
	MaxTraffic     uint64 `json:"max_traffic"`
}
