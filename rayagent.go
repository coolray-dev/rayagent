package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/coolray-dev/rayagent/models"
	"github.com/coolray-dev/rayagent/modules"
	"github.com/coolray-dev/rayagent/utils"
	"github.com/coolray-dev/rayagent/worker"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/status"
)

func main() {

	setupLog()

	// Create a channel to pass Service slice
	schan := make(chan []models.Service, 10)

	// Create a channel to pass signal rayagent process receive
	sigs := make(chan os.Signal)
	// Used to implement gracful shutdown
	signal.Notify(sigs, syscall.SIGINT)

	// Create a waitgroup
	var wg sync.WaitGroup

	// Set worker nodeInfo
	worker.SetToken(modules.Config.GetString("raydash.token"))
	worker.SetID(modules.Config.GetUint64("raydash.nodeID"))

	// Create services receiver
	servicePoller := worker.NewServicePoller(modules.Config.GetString("raydash.url"), modules.Config.GetUint64("raydash.interval"), schan)
	servicePoller.WaitGroup = &wg
	servicePoller.Start()

	// Create a gRPC connection to V2Ray
	gRPCAddr := modules.Config.GetString("v2ray.grpcaddr")
	gRPCConn, err := modules.ConnectGRPC(gRPCAddr, 10*time.Second)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			err = errors.New(s.Message())
		}
		fmt.Printf("connect to gRPC server \"%s\" err: ", gRPCAddr)
	}
	utils.Log.Info("gRPC Connected")

	// Create ServiceClients
	handlerServiceClient := modules.NewHandlerServiceClient(gRPCConn, modules.Config.GetString("v2ray.inbound"))
	statsServiceClient := modules.NewStatsServiceClient(gRPCConn)

	// Get Node Info
	nodeID := modules.Config.GetUint64("raydash.nodeID")
	nodeInfo, infoerr := servicePoller.GetNodeInfo(nodeID)
	if infoerr != nil {
		utils.Log.WithFields(logrus.Fields{
			"error":  infoerr,
			"nodeID": nodeID,
		}).Fatal("Error Getting NodeInfo")
	}

	// Create Services Handler to handler Service slice passed from channel
	serviceHandler := worker.NewServiceHandler(nodeID, nodeInfo, *handlerServiceClient, schan)
	serviceHandler.Tag = modules.Config.GetString("v2ray.inbound")
	serviceHandler.WaitGroup = &wg
	serviceHandler.Start()

	// Create Stats Channel
	statsChannel := make(chan *models.Stats, 10)

	// Create Stats Workers
	statsHandler := worker.NewStatsHandler(statsServiceClient)
	statsHandler.NodeID = nodeID
	statsHandler.NodeInfo = nodeInfo
	statsHandler.Interval = 10
	statsHandler.StatsChannel = statsChannel
	statsHandler.WaitGroup = &wg
	statsHandler.Start() // statsHandler.Start not blocking, no need to use goroutine

	statsSender := worker.NewStatsSender()
	statsSender.RayDashURL = modules.Config.GetString("raydash.url")
	statsSender.Interval = 10
	statsSender.StatsChannel = statsChannel
	statsSender.WaitGroup = &wg
	statsSender.Start()

	// Monitor signal from channel sigs
	go func() {
		sig := <-sigs
		// Do graceful shutdown
		fmt.Println("Shutting down. Caused by", sig)
		fmt.Print("Stopping ServicePoller...")
		servicePoller.Stop()
		fmt.Println("Done")
		serviceHandler.Stop()
		fmt.Print("Stopping Stats Workers...")
		statsSender.Stop()
		statsHandler.Stop()
		fmt.Println("Done")
	}()

	utils.Log.Info("RayAgent Started Successfully")
	// Do not exit until all goroutine done
	wg.Wait()
	return
}

func setupLog() {
	switch modules.Config.GetString("log.level") {
	case "debug":
		utils.Log.SetLevel(logrus.DebugLevel)
	case "info":
		utils.Log.SetLevel(logrus.InfoLevel)
	case "warn":
		utils.Log.SetLevel(logrus.WarnLevel)
	case "error":
		utils.Log.SetLevel(logrus.ErrorLevel)
	default:
		utils.Log.SetLevel(logrus.InfoLevel)
	}

}
