package modules

import (
	"errors"
	"strings"

	"github.com/coolray-dev/rayagent/utils"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config is a viper instance
// We use single viper instance in rayagent
var Config *viper.Viper

func init() {
	Config = viper.New()
	pflag.String("config", "config", "config file name")
	pflag.Parse()
	Config.BindPFlags(pflag.CommandLine)
	if Config.IsSet("config") {
		Config.SetConfigFile(Config.GetString("config"))
	} else {
		Config.SetConfigName("config")
		Config.SetConfigType("yaml")
		Config.AddConfigPath(utils.AbsPath(""))
		Config.AddConfigPath("/etc/rayagent")
	}
	Config.SetEnvPrefix("RAYAGENT")
	Config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	Config.AutomaticEnv()

	if err := Config.ReadInConfig(); err != nil {
		utils.Log.WithError(err).Fatal("Fatal Error Reading Config File")
	}
	Config.WatchConfig()

	// Check if neccessary config is set
	if err := checkConfig(); err != nil {
		utils.Log.Fatal("Config Error")
	}
	setDefault()
	return
}

func checkConfig() error {
	if !Config.IsSet("raydash.nodeID") {
		utils.Log.Error("raydash nodeID not set")
		return errors.New("raydash nodeID not set")
	}
	if !Config.IsSet("raydash.url") {
		utils.Log.Error("raydash URL not set")
		return errors.New("raydash URL not set")
	}
	if !Config.IsSet("v2ray.grpcaddr") {
		utils.Log.Error("v2ray gRPC address not set")
		return errors.New("v2ray gRPC address not set")
	}
	return nil
}

func setDefault() {
	Config.SetDefault("raydash.interval", 5)
	Config.SetDefault("v2ray.inbound", "rayagent")
}
