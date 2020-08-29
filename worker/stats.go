package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coolray-dev/rayagent/models"
	"github.com/coolray-dev/rayagent/modules"
	"github.com/coolray-dev/rayagent/utils"
)

// StatsHandler is a handler get traffic stats from v2ray and report back to raydash
type StatsHandler struct {
	NodeID             uint64
	NodeInfo           *models.Node
	statsServiceClient *modules.StatsServiceClient
	users              map[string]*models.User
	Ticker             *time.Ticker
	Interval           uint64 // interval in second
	StatsChannel       chan *models.Stats
	WaitGroup          *sync.WaitGroup
	lock               *sync.RWMutex
}

// NewStatsHandler returns a ptr of StatsHandler instance
func NewStatsHandler(sc *modules.StatsServiceClient) *StatsHandler {
	return &StatsHandler{
		users:              userPool,
		statsServiceClient: sc,
		lock:               &userPoolLock,
	}
}

// Start start statshandler instance
func (s *StatsHandler) Start() {
	s.WaitGroup.Add(1)
	s.startTicker(s.getStats)
	utils.Log.Info("StatsHandler Started")
	return
}

// Stop do graceful shutdown and close the goroutine
func (s *StatsHandler) Stop() {
	s.Ticker.Stop()
	s.WaitGroup.Done()
	return
}

func (s *StatsHandler) getStats() {
	s.lock.RLock()
	for _, u := range s.users {

		var stats models.Stats
		var err error
		stats.Traffic, err = s.statsServiceClient.GetUserTraffic(u.Email)
		if err != nil {
			continue
		}
		stats.Email = u.Email

		s.StatsChannel <- &stats
	}
	s.lock.RUnlock()
	utils.Log.Debug("Successfully procceed all user stats")
	return
}

func (s *StatsHandler) startTicker(worker func()) {
	ticker := time.NewTicker(time.Second * time.Duration(s.Interval))
	go func() {
		for range ticker.C {
			worker()
		}
	}()
	s.Ticker = ticker
	return
}

// StatsSender receive stats struct from channel and send it to raydash
type StatsSender struct {
	RayDashURL   string // e.g. https://raydash.example.com
	Interval     uint64 // interval in second
	Ticker       *time.Ticker
	users        map[string]*models.User
	StatsChannel chan *models.Stats
	WaitGroup    *sync.WaitGroup
	lock         *sync.RWMutex
	httpClient   *http.Client
}

// NewStatsSender returns a ptr of StatsSender instance
func NewStatsSender() *StatsSender {
	return &StatsSender{
		users:      userPool,
		lock:       &userPoolLock,
		httpClient: createHTTPClient(),
	}
}

// Start start the instance
func (s *StatsSender) Start() {
	s.WaitGroup.Add(1)
	go s.syncStats()
	utils.Log.Info("StatsSender Started")
	return
}

// Stop stop the instance
func (s *StatsSender) Stop() {
	close(s.StatsChannel)
	s.WaitGroup.Done()
	return
}

func (s *StatsSender) syncStats() {
	for stats := range s.StatsChannel {
		// pull user
		if stats.Traffic != 0 {
			s.lock.Lock()
			s.users[stats.Email].CurrentTraffic += stats.Traffic
			s.lock.Unlock()

			s.lock.RLock()
			s.patchUser(s.users[stats.Email])
			s.lock.RUnlock()
		}

	}
}

func (s *StatsSender) patchUser(u *models.User) error {
	jsonStr, _ := json.Marshal(u)
	req, _ := http.NewRequest(http.MethodPatch, s.RayDashURL+"/users/"+u.Username, bytes.NewBuffer([]byte(jsonStr)))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := s.httpClient.Do(req)
	if resp.StatusCode != http.StatusOK {
		return errors.New("Error Sending API Call to" + s.RayDashURL + "/users/" + u.Username)
	}
	return nil
}
