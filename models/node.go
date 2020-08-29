package models

// Node is a struct of node info
type Node struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Host           string `json:"host"` // The Host to access v2ray
	Ports          string `json:"ports"`
	AccessKey      string `json:"access_key"`
	CurrentTraffic uint64 `json:"current_traffic"`
	MaxTraffic     uint64 `json:"max_traffic"`
	HasUDP         bool   `json:"hasUDP"`
	HasMultiPort   bool   `json:"hasMultiPort"`
	Settings       `json:"settings"`
}

type Settings struct {
	Listen             string `json:"listen"`
	Port               uint   `json:"port"`
	VmessSetting       `json:"vmessSettings"`
	ShadowsocksSetting `json:"shadowsocksSettings"`
}
