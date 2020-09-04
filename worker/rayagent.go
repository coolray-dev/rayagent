package worker

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coolray-dev/rayagent/models"
	"github.com/coolray-dev/rayagent/modules"
	"github.com/coolray-dev/rayagent/utils"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// RayAgent instance
type RayAgent struct {
	nodeID         uint64
	nodeInfo       *models.Node
	servicePoller  *ServicePoller
	serviceHandler *ServiceHandler
	statsHandler   *StatsHandler
	statsSender    *StatsSender
	waitGroup      *sync.WaitGroup
	schan          chan []models.Service
	statsChannel   chan *models.Stats
	gRPCConn       *grpc.ClientConn
}

// NewRayAgent return a rayagent
func NewRayAgent(wg *sync.WaitGroup) *RayAgent {
	return &RayAgent{
		waitGroup:    wg,
		schan:        make(chan []models.Service, 10),
		statsChannel: make(chan *models.Stats, 10),
	}
}

// Start start a rayagent
func (r *RayAgent) Start() {

	// Set worker nodeInfo
	nodeInfo.Token = modules.Config.GetString("raydash.token")
	nodeInfo.ID = modules.Config.GetUint64("raydash.nodeID")
	initUserPool()
	r.startServicePoller()

	r.startV2RayConnection()
	// Create ServiceClients
	handlerServiceClient := modules.NewHandlerServiceClient(r.gRPCConn, modules.Config.GetString("v2ray.inbound"))
	statsServiceClient := modules.NewStatsServiceClient(r.gRPCConn)
	r.getNodeInfo()
	r.startServiceHandler(handlerServiceClient)

	r.startStatsHandler(statsServiceClient)
	r.startStatsSender()

	utils.Log.Info("RayAgent Started Successfully")
	return
}

// Stop stop a rayagent
func (r *RayAgent) Stop() {
	fmt.Print("Stopping ServicePoller...")
	r.servicePoller.Stop()
	fmt.Println("Done")
	r.serviceHandler.Stop()
	fmt.Print("Stopping Stats Workers...")
	r.statsSender.Stop()
	r.statsHandler.Stop()
	fmt.Println("Done")
}

func (r *RayAgent) startServicePoller() {

	r.servicePoller = NewServicePoller(modules.Config.GetString("raydash.url"),
		modules.Config.GetUint64("raydash.interval"),
		r.schan)
	r.servicePoller.WaitGroup = r.waitGroup
	r.servicePoller.Start()
}

func (r *RayAgent) startServiceHandler(hc *modules.HandlerServiceClient) {
	// Create Services Handler to handler Service slice passed from channel
	r.serviceHandler = NewServiceHandler(r.nodeID, r.nodeInfo, *hc, r.schan)
	r.serviceHandler.Tag = modules.Config.GetString("v2ray.inbound")
	r.serviceHandler.WaitGroup = r.waitGroup
	r.serviceHandler.Start()
}

func (r *RayAgent) startStatsHandler(sc *modules.StatsServiceClient) {
	// Create Stats Workers
	r.statsHandler = NewStatsHandler(sc)
	r.statsHandler.NodeID = r.nodeID
	r.statsHandler.NodeInfo = r.nodeInfo
	r.statsHandler.Interval = 10
	r.statsHandler.StatsChannel = r.statsChannel
	r.statsHandler.WaitGroup = r.waitGroup
	r.statsHandler.Start() // statsHandler.Start not blocking, no need to use goroutine
}

func (r *RayAgent) startStatsSender() {
	r.statsSender = NewStatsSender()
	r.statsSender.RayDashURL = modules.Config.GetString("raydash.url")
	r.statsSender.Interval = 10
	r.statsSender.StatsChannel = r.statsChannel
	r.statsSender.WaitGroup = r.waitGroup
	r.statsSender.Start()
}

func (r *RayAgent) startV2RayConnection() {
	gRPCAddr := modules.Config.GetString("v2ray.grpcaddr")
	var err error
	r.gRPCConn, err = modules.ConnectGRPC(gRPCAddr, 10*time.Second)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			err = errors.New(s.Message())
		}
		fmt.Printf("connect to gRPC server \"%s\" err: ", gRPCAddr)
	}
	utils.Log.Info("gRPC Connected")
}

func (r *RayAgent) getNodeInfo() {
	r.nodeID = modules.Config.GetUint64("raydash.nodeID")
	var err error
	r.nodeInfo, err = r.servicePoller.GetNodeInfo(r.nodeID)
	if err != nil {
		utils.Log.WithFields(logrus.Fields{
			"error":  err,
			"nodeID": r.nodeID,
		}).Fatal("Error Getting NodeInfo")
	}
}
