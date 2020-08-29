package modules

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	statsservice "v2ray.com/core/app/stats/command"
)

type StatsServiceClient struct {
	statsservice.StatsServiceClient
}

func NewStatsServiceClient(client *grpc.ClientConn) *StatsServiceClient {
	return &StatsServiceClient{
		StatsServiceClient: statsservice.NewStatsServiceClient(client),
	}
}

func (s *StatsServiceClient) GetUserTraffic(email string) (uint64, error) {
	up, err := s.getUserUplink(email)
	if err != nil {
		return 0, err
	}
	down, err2 := s.getUserDownlink(email)
	if err2 != nil {
		return 0, err2
	}
	return up + down, nil

}

func (s *StatsServiceClient) getUserUplink(email string) (uint64, error) {
	return s.getUserStats(fmt.Sprintf("user>>>%s>>>traffic>>>uplink", email), true)
}

func (s *StatsServiceClient) getUserDownlink(email string) (uint64, error) {
	return s.getUserStats(fmt.Sprintf("user>>>%s>>>traffic>>>downlink", email), true)
}

func (s *StatsServiceClient) getUserStats(name string, reset bool) (uint64, error) {
	req := &statsservice.GetStatsRequest{
		Name:   name,
		Reset_: reset,
	}

	res, err := s.GetStats(context.Background(), req)
	if err != nil {
		if status, ok := status.FromError(err); ok && strings.HasSuffix(status.Message(), fmt.Sprintf("%s not found.", name)) {
			return 0, nil
		}

		return 0, err
	}

	return uint64(res.Stat.Value), nil
}
