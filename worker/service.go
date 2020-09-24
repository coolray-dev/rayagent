package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/coolray-dev/rayagent/models"
	"github.com/coolray-dev/rayagent/modules"
	"github.com/coolray-dev/rayagent/utils"
	"github.com/sirupsen/logrus"
	"v2ray.com/core"
	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/serial"
	vmessInbound "v2ray.com/core/proxy/vmess/inbound"
	"v2ray.com/core/transport/internet"
	"v2ray.com/core/transport/internet/websocket"
)

// ServicePoller get services from raydash
type ServicePoller struct {
	RayDashURL     string // e.g. https://raydash.example.com
	Interval       uint64 // interval in second
	Ticker         *time.Ticker
	ServiceChannel chan<- []models.Service
	WaitGroup      *sync.WaitGroup
	httpClient     *http.Client
}

// NewServicePoller return a new ServicePoller with private sub set
func NewServicePoller(url string, interval uint64, schan chan<- []models.Service) *ServicePoller {
	return &ServicePoller{
		RayDashURL:     url,
		Interval:       interval,
		ServiceChannel: schan,
		httpClient:     createHTTPClient(),
	}
}

// Start start a instance
func (c *ServicePoller) Start() {
	c.WaitGroup.Add(1)
	c.startTicker(c.getServices)
	utils.Log.Info("ServicePoller Started")
	return
}

// Stop stop a instance
func (c *ServicePoller) Stop() {
	c.stopTicker()
	close(c.ServiceChannel)
	c.WaitGroup.Done()
	return
}

// GetNodeInfo is a general func retrieve node info from /nodes/:id
func (c *ServicePoller) GetNodeInfo(nodeID uint64) (*models.Node, error) {

	// Generate Request
	endpoint := c.RayDashURL + "/nodes/" + strconv.Itoa(int(nodeID))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+"node."+nodeInfo.Token)

	// Call API
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error Calling RayDash API: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Error Calling RayDash API: Code %d", response.StatusCode)
	}

	// Read Response
	body, _ := ioutil.ReadAll(response.Body)

	// Bind Response
	type Response struct {
		Node models.Node `json:"node"`
	}
	var resp Response
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, fmt.Errorf("Error Unmarshalling Response: %w", err)
	}
	return &resp.Node, nil
}

// getServices call raydash api
func (c *ServicePoller) getServices() {

	// Generate Request
	endpoint := c.RayDashURL + "/nodes/" + fmt.Sprint(nodeInfo.ID) + "/services"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+"node."+nodeInfo.Token)

	// Call API
	response, err := c.httpClient.Do(req)
	if err != nil {
		utils.Log.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Error Calling RayDash API")
		return
	}
	if response.StatusCode != http.StatusOK {
		utils.Log.WithField("StatusCode", response.StatusCode).Error("Error Calling RayDash API")
		return
	}
	utils.Log.Debug("Successfully Called RayDash API:" + endpoint)
	body, _ := ioutil.ReadAll(response.Body)

	// Unmarshal service from response
	type services struct {
		Services []models.Service `json:"services"`
		Total    uint64           `json:"total"`
	}
	var s services
	err = json.Unmarshal(body, &s)
	if err != nil {
		utils.Log.WithError(err).Error("Error Parsing RayDash API Response")
	}

	// Validate services before sending it into channel
	for _, i := range s.Services {
		if err := modules.Validator.Struct(&i); err != nil {
			utils.Log.WithError(err).Warn("Service Validation Failed")
			return
		}
	}

	c.ServiceChannel <- s.Services
	return
}

func (c *ServicePoller) startTicker(worker func()) {
	ticker := time.NewTicker(time.Second * time.Duration(c.Interval))
	go func() {
		for range ticker.C {
			worker()
		}
	}()
	c.Ticker = ticker
	return
}

func (c *ServicePoller) stopTicker() {
	c.Ticker.Stop()
	return
}

// ServiceHandler is a handler controls incoming services info and talk to v2ray through handlerServiceClient
type ServiceHandler struct {
	NodeID               uint64
	NodeInfo             *models.Node
	Tag                  string
	handlerServiceClient modules.HandlerServiceClient
	users                map[string]*models.User // Access worker public user pool
	Services             []models.Service
	ServicesChannel      <-chan []models.Service
	WaitGroup            *sync.WaitGroup
	lock                 *sync.RWMutex
}

// NewServiceHandler return a pointer to service handler using provided info
func NewServiceHandler(nodeID uint64,
	nodeInfo *models.Node,
	hc modules.HandlerServiceClient,
	schan <-chan []models.Service) *ServiceHandler {
	return &ServiceHandler{
		NodeID:               nodeID,
		NodeInfo:             nodeInfo,
		handlerServiceClient: hc,
		users:                userPool, // userpool shared within worker package
		Services:             make([]models.Service, 0),
		ServicesChannel:      schan,
		lock:                 &userPoolLock,
	}
}

// Start start a instance
func (h *ServiceHandler) Start() {
	h.WaitGroup.Add(1)
	go h.syncServices()
	utils.Log.Info("ServiceHandler Started")
	return
}

// Stop stop a instance
func (h *ServiceHandler) Stop() {
	h.WaitGroup.Done()
	return
}

func (h *ServiceHandler) syncServices() {
	if h.NodeInfo.HasMultiPort {
		h.syncServicesMultiInbound()
		return
	}
	h.syncServicesSingleInbound()
	return
}

func (h *ServiceHandler) syncServicesMultiInbound() {
	for services := range h.ServicesChannel {
		// Calculate Services to add
		ServicesToAdd := sub(services, h.Services).([]models.Service)
		// Calculate Services to remove
		ServicesToDel := sub(h.Services, services).([]models.Service)

		// Perform add and delete
		for i, s := range ServicesToAdd {
			_ = h.handlerServiceClient.AddInbound(utils.ConvertVmessInbound(&s))
			h.Services = append(h.Services, ServicesToAdd[i])
		}
		for i, s := range ServicesToDel {
			j := findServiceIndex(&ServicesToDel[i], h.Services)
			_ = h.handlerServiceClient.RemoveInbound(fmt.Sprintf("%x", s.ID))
			h.Services = append(h.Services[:j], h.Services[j+1:]...)
		}
	}
}

func (h *ServiceHandler) syncServicesSingleInbound() {
	// Init
	if err := h.initializeSingleInbound(); err != nil {
		utils.Log.Fatal("Error Initializing Single Inbound RayAgent")
		return
	}

	// Start listening []Services from channel
	for services := range h.ServicesChannel {
		// Calculate Services to add
		ServicesToAdd := sub(services, h.Services).([]models.Service)
		// Calculate Services to remove
		ServicesToDel := sub(h.Services, services).([]models.Service)

		// Deal with users excceeded their traffic
		var tmp []models.Service
		// Filter ServiceToAdd
		for i := range ServicesToAdd {
			if h.users[ServicesToAdd[i].Email].MaxTraffic >= h.users[ServicesToAdd[i].Email].CurrentTraffic {
				tmp = append(tmp, ServicesToAdd[i])
			}
		}
		ServicesToAdd = tmp
		// Add to ServiceToDel
		for i := range h.Services {
			if h.users[h.Services[i].Email].MaxTraffic <= h.users[h.Services[i].Email].CurrentTraffic {
				ServicesToDel = append(ServicesToDel, h.Services[i])
			}
		}

		// Convert Services into v2ray user
		UsersToAdd := make([]protocol.User, 0)
		for _, s := range ServicesToAdd {
			UsersToAdd = append(UsersToAdd, *utils.ConvertService(&s))
		}
		UsersToDel := make([]protocol.User, 0)
		for _, s := range ServicesToDel {
			UsersToDel = append(UsersToDel, *utils.ConvertService(&s))
		}
		// Perform add and delete
		for i, u := range UsersToAdd {
			if err := h.handlerServiceClient.AddUser(&u); err != nil {
				utils.Log.Errorf("Error Adding User %s", u.Email)
			}
			utils.Log.Infof("Successfully Added User %s", u.Email)
			h.Services = append(h.Services, ServicesToAdd[i])
		}
		for i, u := range UsersToDel {
			j := findServiceIndex(&ServicesToDel[i], h.Services)
			if err := h.handlerServiceClient.DelUser(u.Email); err != nil {
				utils.Log.Errorf("Error Deleting User %s", u.Email)
			}
			utils.Log.Infof("Successfully Deleted User %s", u.Email)
			h.Services = append(h.Services[:j], h.Services[j+1:]...)
		}

	}
	return
}

func (h *ServiceHandler) initializeSingleInbound() error {
	// Clear target inbound
	if err := h.handlerServiceClient.RemoveInbound(h.Tag); err != nil {
		utils.Log.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Error Removing Inbound")
		return err
	}

	// Re-Add inbound with the same tag
	inboundHandlerConfig, err := h.genVmessInbound()
	if err != nil {
		utils.Log.WithError(err).Error("Error Generating VmessInbound")
		return err
	}
	if err = h.handlerServiceClient.AddInbound(inboundHandlerConfig); err != nil {
		utils.Log.WithError(err).Error("Error Adding Inbound")
		return errors.New("Error Adding Inbound")
	}
	return nil
}

// genVmessInbound generate a vmess inbound with NO USER
func (h *ServiceHandler) genVmessInbound() (*core.InboundHandlerConfig, error) {
	// Get port from nodeinfo and validate
	port, err := net.PortFromInt(uint32(h.NodeInfo.Port))
	if err != nil {
		utils.Log.WithFields(logrus.Fields{
			"error": err.Error(),
			"port":  h.NodeInfo.Port,
		}).Error("Invalid Port")
		return nil, errors.New("Invalid Port")
	}

	// Validate protocol
	// Must validate because h.protocolConfig does not check
	if !isProtocolValid(h.NodeInfo.VmessSetting.StreamSettings.TransportProtocol) {
		utils.Log.WithFields(logrus.Fields{
			"protocol": h.NodeInfo.TransportProtocol,
		}).Error("Invalid Protocol")
		return nil, errors.New("Invalid Protocol")
	}
	config := &core.InboundHandlerConfig{
		Tag: h.Tag,
		ReceiverSettings: serial.ToTypedMessage(
			&proxyman.ReceiverConfig{
				PortRange: net.SinglePortRange(port),
				Listen:    net.NewIPOrDomain(net.ParseAddress("0.0.0.0")), // hard coded
				AllocationStrategy: &proxyman.AllocationStrategy{
					Type: proxyman.AllocationStrategy_Always,
				}, // Must have
				StreamSettings: &internet.StreamConfig{
					ProtocolName: h.NodeInfo.VmessSetting.StreamSettings.TransportProtocol,
					TransportSettings: []*internet.TransportConfig{&internet.TransportConfig{
						ProtocolName: h.NodeInfo.VmessSetting.StreamSettings.TransportProtocol,
						Settings:     h.protocolConfig(),
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
				User: []*protocol.User{},
				Default: &vmessInbound.DefaultConfig{
					AlterId: 64, // hard coded
				},
				//Detour: &vmessInbound.DetourConfig{
				//	To: "", // No detour, not support yet
				//},
				SecureEncryptionOnly: true,
			},
		),
	}
	return config, nil
}

// Return a compatible format of protocol config
func (h *ServiceHandler) protocolConfig() *serial.TypedMessage {
	switch h.NodeInfo.VmessSetting.StreamSettings.TransportProtocol {
	case "websocket":
		return serial.ToTypedMessage(&websocket.Config{
			Path: "",
		})
	default:
		return nil

	}
}

// Check if protocol name is compatible with vmess
func isProtocolValid(p string) bool {
	var validprotocol map[string]bool = map[string]bool{
		"tcp":          true,
		"websocket":    true,
		"http":         true,
		"mkcp":         true,
		"domainsocket": true,
		"quic":         true,
	}
	_, found := validprotocol[p]
	return found
}

func findServiceIndex(s *models.Service, array []models.Service) int {
	for i, service := range array {
		if service == *s {
			return i
		}
	}
	return -1
}

// sub return element in A not in B
// use hash map, so O(n)
// use reflact, so not super efficient
func sub(A interface{}, B interface{}) interface{} {

	hash := make(map[interface{}]bool)
	av := reflect.ValueOf(A)
	bv := reflect.ValueOf(B)
	aType := reflect.TypeOf(A)
	set := reflect.Zero(aType)
	for i := 0; i < bv.Len(); i++ {
		el := bv.Index(i).Interface()
		hash[el] = true
	}

	for i := 0; i < av.Len(); i++ {
		el := av.Index(i)
		if _, found := hash[el.Interface()]; !found {
			set = reflect.Append(set, el)
		}
	}

	return set.Interface()
}
