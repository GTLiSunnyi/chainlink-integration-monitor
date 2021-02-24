package daemon

import (
	"os"
	"os/signal"
	"time"

	"github.com/spf13/viper"

	"github.com/secret2830/chainlink-integration-monitor/base"
	"github.com/secret2830/chainlink-integration-monitor/common"
	"github.com/secret2830/chainlink-integration-monitor/monitors"
)

type Application struct {
	Monitors []base.IMonitor
}

func NewApplication(config *viper.Viper) *Application {
	return &Application{
		Monitors: []base.IMonitor{
			newIRISHUBMonitor(config),
		},
	}
}

func (app *Application) Start() {
	for _, monitor := range app.Monitors {
		go monitor.Start()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig

	common.Logger.Info("Stopping the monitor...")

	app.Stop()
	os.Exit(0)
}

func (app *Application) Stop() {
	for _, monitor := range app.Monitors {
		monitor.Stop()
	}

	common.Logger.Info("All monitors stopped")
}

func newIRISHUBMonitor(config *viper.Viper) *monitors.Monitor {
	rpcURL := config.GetString("irishub.rpc_endpoint")
	gRPCURL := config.GetString("irishub.grpc_endpoint")
	prometheusAddr := config.GetString("irishub.prometheus_addr")
	interval := config.GetInt64("irishub.interval")
	providerAddrs := config.GetStringSlice("irishub.provider_addresses")
	threshold := config.GetInt64("balance.threshold")

	rpcEndpoint := base.NewEndpointFromURL(rpcURL)
	grpcEndpoint := base.NewEndpointFromURL(gRPCURL)

	return monitors.NewMonitor(
		rpcEndpoint,
		grpcEndpoint,
		prometheusAddr,
		time.Duration(interval)*time.Second,
		threshold,
		providerAddrs,
	)
}
