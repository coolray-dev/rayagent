package utils

import (
	"github.com/coolray-dev/rayagent/models"
	"v2ray.com/core"
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/serial"
	"v2ray.com/core/proxy/vmess"
	vmessInbound "v2ray.com/core/proxy/vmess/inbound"
	"v2ray.com/core/transport/internet"
	"v2ray.com/core/transport/internet/websocket"
)

func ConvertVmessInbound(s *models.Service) *core.InboundHandlerConfig {
	port, _ := net.PortFromInt(uint32(s.Port))
	return &core.InboundHandlerConfig{
		Tag: string(s.ID),
		ReceiverSettings: serial.ToTypedMessage(
			&proxyman.ReceiverConfig{
				PortRange: net.SinglePortRange(port),

				Listen: net.NewIPOrDomain(net.ParseAddress("0.0.0.0")),
				AllocationStrategy: &proxyman.AllocationStrategy{
					Type: proxyman.AllocationStrategy_Always,
				},
				StreamSettings: &internet.StreamConfig{
					Protocol:     internet.TransportProtocol(internet.TransportProtocol_value[s.VmessSetting.StreamSettings.TransportProtocol]),
					ProtocolName: s.VmessSetting.StreamSettings.TransportProtocol,
					TransportSettings: []*internet.TransportConfig{&internet.TransportConfig{
						Protocol:     internet.TransportProtocol(internet.TransportProtocol_value[s.VmessSetting.StreamSettings.TransportProtocol]),
						ProtocolName: s.VmessSetting.StreamSettings.TransportProtocol,
						Settings: // need a func to handler different transport settings
						serial.ToTypedMessage(&websocket.Config{
							Path: "", // leave for future
						}),
					}},
				},
				ReceiveOriginalDestination: true,
				SniffingSettings: &proxyman.SniffingConfig{
					Enabled:             true,
					DestinationOverride: []string{"http", "tls"},
				},
			},
		),
		ProxySettings: serial.ToTypedMessage(
			&vmessInbound.Config{
				User: []*protocol.User{ConvertService(s)},
				Default: &vmessInbound.DefaultConfig{
					AlterId: 64, // hard coded
				},
				Detour: &vmessInbound.DetourConfig{
					To: "", // No detour, not support yet
				},
				SecureEncryptionOnly: true,
			},
		),
	}
}

func ConvertService(s *models.Service) *protocol.User {
	return &protocol.User{
		Level: 0,
		Email: s.Email,
		Account: serial.ToTypedMessage(&vmess.Account{
			Id:      s.UUID,
			AlterId: 64, // hard coded
		}),
	}
}
