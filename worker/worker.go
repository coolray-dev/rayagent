package worker

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coolray-dev/rayagent/models"
	"github.com/coolray-dev/rayagent/modules"
	"github.com/coolray-dev/rayagent/utils"
	"github.com/sirupsen/logrus"
)

var userPool map[string]*models.User

var userPoolLock sync.RWMutex

var nodeToken string

func SetToken(t string) {
	nodeToken = t
}

func init() {
	initUserPool()
}

func initUserPool() {
	// Initialize userPool
	userPool = make(map[string]*models.User)

	// Set retry times to 3 before panic
	retries := 3
	var err error
	var response *http.Response
	for retries > 0 {
		response, err = http.Get(modules.Config.GetString("raydash.url") + "/users")
		if err != nil {
			retries--
			utils.Log.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("Error Calling RayDash API")
		} else {
			break
		}
	}

	// Exit rayagent if still cannot retrieve users
	if err != nil {
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
