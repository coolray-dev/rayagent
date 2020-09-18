package worker

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/coolray-dev/rayagent/models"
	"github.com/coolray-dev/rayagent/modules"
	"github.com/coolray-dev/rayagent/utils"
	"github.com/sirupsen/logrus"
)

var userPool map[string]*models.User

var userPoolLock sync.RWMutex

type typeNodeInfo struct {
	Token string
	ID    uint64
}

var nodeInfo typeNodeInfo

func init() {
	// Initialize userPool
	userPool = make(map[string]*models.User)
}

func initUserPool() {

	// Set retry times to 3 before exit program
	retries := 3
	var err error
	var response *http.Response
	for retries > 0 {

		// Generate Request
		endpoint := modules.Config.GetString("raydash.url") + "/nodes/" + strconv.Itoa(int(modules.Config.GetUint64("raydash.nodeID"))) + "/users"
		var req *http.Request
		req, err = http.NewRequest(http.MethodGet, endpoint, nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+"node."+modules.Config.GetString("raydash.token"))

		// Call API
		client := http.DefaultClient
		response, err = client.Do(req)
		if err != nil {
			retries--
			utils.Log.WithError(err).Error("Error Calling RayDash API")
		} else if response.StatusCode != http.StatusOK {
			retries--
			utils.Log.WithField("StatusCode", response.StatusCode).Error("Error Calling RayDash API")
		} else {
			break
		}
	}

	// Exit rayagent if still cannot retrieve users
	if err != nil || response.StatusCode != http.StatusOK {
		utils.Log.Fatal("Error Calling RayDash API")
	}

	// Read and bind response
	body, _ := ioutil.ReadAll(response.Body)
	type Response struct {
		Users []models.User `json:"users" binding:"required"`
	}
	var r Response
	err = json.Unmarshal(body, &r)
	if err != nil {
		utils.Log.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Panic("Error Parsing RayDash API Response")
	}

	// Fill userPool
	for _, u := range r.Users {

		// Validate user before adding
		if err := modules.Validator.Struct(&u); err != nil {
			utils.Log.WithField("username", u.Username).WithError(err).Warn("User Validation Failed")
			continue
		}

		userPool[u.Email] = &u
	}
	return
}

// Network Const
const (
	MaxIdleConns        int = 10
	MaxIdleConnsPerHost int = 10
	IdleConnTimeout     int = 90
)

// createHTTPClient for connection re-use
func createHTTPClient() *http.Client {
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        MaxIdleConns,
			MaxIdleConnsPerHost: MaxIdleConnsPerHost,
			IdleConnTimeout:     time.Duration(IdleConnTimeout) * time.Second,
		},
		Timeout: 30 * time.Second,
	}
	return client
}
