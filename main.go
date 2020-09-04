package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/coolray-dev/rayagent/modules"
	"github.com/coolray-dev/rayagent/utils"
	"github.com/coolray-dev/rayagent/worker"
	"github.com/sirupsen/logrus"
)

func main() {

	setupLog()

	// Create a channel to pass signal rayagent process receive
	sigs := make(chan os.Signal)
	// Used to implement gracful shutdown
	signal.Notify(sigs, syscall.SIGINT)

	// Create a waitgroup
	var wg sync.WaitGroup

	// Setup RayAgent
	rayagent := worker.NewRayAgent(&wg)
	rayagent.Start()

	// Monitor signal from channel sigs
	go func() {
		sig := <-sigs
		// Do graceful shutdown
		fmt.Println("Shutting down. Caused by", sig)
		rayagent.Stop()
	}()

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
