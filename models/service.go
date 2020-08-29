package models

import "time"

type Service struct {
	ID        uint64    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Name        string `json:"name"`
	Description string `json:"description"`
	UserID      uint64
	NodeID      uint64

	Host      string `json:"host"`
	Port      uint   `json:"port"`
	Protocol  string `json:"protocol"`
	VmessUser `json:"vmessUser"`
	VmessSetting
	ShadowsocksSetting
}

type ShadowsocksSetting struct{}

type VmessUser struct {
	Email    string `json:"email"`
	UUID     string `json:"uuid"`
	AlterID  uint   `json:"alterid"`
	Security string `json:"security"`
}
type VmessSetting struct {
	StreamSettings `json:"streamSettings"`
	SniffingSettings
	allocate struct{}
}

type StreamSettings struct {
	TransportProtocol string `json:"protocol"`
}

type SniffingSettings struct{}
