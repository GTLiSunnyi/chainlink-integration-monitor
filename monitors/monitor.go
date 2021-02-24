package monitors

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	abci "github.com/tendermint/tendermint/abci/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"

	servicesdk "github.com/irisnet/service-sdk-go"
	"github.com/irisnet/service-sdk-go/types"

	"github.com/secret2830/chainlink-integration-monitor/base"
	"github.com/secret2830/chainlink-integration-monitor/common"
)

var _ base.IMonitor = &Monitor{}

var (
	ErrCnt = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "monitors",
			Help: "",
		},
		[]string{"err:"},
	)
)

const (
	ServiceSlashingEventType = "service_slash"
)

func startListner(addr string) {
	// Register the summary and the histogram with Prometheus's default registry.
	prometheus.MustRegister(ErrCnt)
	srv := &http.Server{
		Addr: addr,
		Handler: promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer, promhttp.HandlerFor(
				prometheus.DefaultGatherer,
				promhttp.HandlerOpts{MaxRequestsInFlight: 10},
			),
		),
	}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// Error starting or closing listener:
			ErrCnt.WithLabelValues("Prometheus HTTP server ListenAndServe err: ", fmt.Sprintf("%s", err))
			common.Logger.Error("Prometheus HTTP server ListenAndServe", "err", err)

		}
	}()
}

type Monitor struct {
	Client            servicesdk.ServiceClient
	RPCEndpoint       base.Endpoint
	GRPCEndpoint      base.Endpoint
	Interval          time.Duration
	Threshold         int64
	ProviderAddresses map[string]bool
	lastHeight        int64
	stopped           bool
}

func NewMonitor(
	rpcEndpoint base.Endpoint,
	grpcEndpoint base.Endpoint,
	prometheusAddr string,
	interval time.Duration,
	threshold int64,
	providerAddresses []string,
) *Monitor {
	cfg := types.ClientConfig{
		NodeURI:  rpcEndpoint.URL,
		GRPCAddr: grpcEndpoint.URL,
	}
	serviceClient := servicesdk.NewServiceClient(cfg)

	addressMap := make(map[string]bool)
	for _, addr := range providerAddresses {
		addressMap[addr] = true
	}

	startListner(prometheusAddr)

	return &Monitor{
		Client:            serviceClient,
		RPCEndpoint:       rpcEndpoint,
		GRPCEndpoint:      grpcEndpoint,
		Interval:          interval,
		Threshold: threshold,
		ProviderAddresses: addressMap,
	}
}

func (m *Monitor) Start() {
	common.Logger.Infof("monitor started, provider addresses: %v", m.ProviderAddresses)

	for {
		m.scan()

		if !m.stopped {
			time.Sleep(m.Interval)
			continue
		}

		return
	}
}

func (m *Monitor) scan() {
	currentHeight, err := m.getLatestHeight()
	if err != nil {
		common.Logger.Warnf("failed to retrieve the latest block height: %s", err)
		return
	}

	common.Logger.Infof("block height: %d", currentHeight)

	if m.lastHeight == 0 {
		m.lastHeight = currentHeight - 1
	}

	m.scanByRange(m.lastHeight+1, currentHeight)
}

func (m Monitor) getLatestHeight() (int64, error) {
	res, err := m.Client.Status(context.Background())
	if err != nil {
		return -1, err
	}

	return res.SyncInfo.LatestBlockHeight, nil
}

func (m *Monitor) scanByRange(startHeight int64, endHeight int64) {
	for h := startHeight; h <= endHeight; h++ {
		_, err := m.Client.BlockResults(context.Background(), &h)
		if err != nil {
			common.Logger.Warnf("failed to retrieve the block result, height: %d, err: %s", h, err)
			continue
		}
	}

	for addr := range m.ProviderAddresses {
		for h := startHeight; h <= endHeight; h++ {
			blockResult, err := m.Client.BlockResults(context.Background(), &h)
			if err != nil {
				common.Logger.Warnf("failed to retrieve the block result, height: %d, err: %s", h, err)
				continue
			}
			m.parseSlashEvents(blockResult)
		}
		m.lastHeight = endHeight

		baseAccount, err := m.Client.QueryAccount(addr)
		if err != nil {
			common.Logger.Errorf("failed to query balance, err: %s", err)
		}
		isLTE := baseAccount.Coins.IsAllLTE(types.NewCoins(types.NewCoin(baseAccount.Coins.GetDenomByIndex(0), types.NewInt(m.Threshold))))
		if isLTE {
			ErrCnt.WithLabelValues("balance of address(", addr, ") is almost empty!")
			common.Logger.Warnf("balance of address(%s) is almost empty!", addr)
		}
	}

	m.lastHeight = endHeight
}

func (m *Monitor) parseSlashEvents(blockResult *ctypes.ResultBlockResults) {
	if len(blockResult.TxsResults) > 0 {
		m.parseSlashEventsFromTxs(blockResult.TxsResults)
	}

	if len(blockResult.EndBlockEvents) > 0 {
		m.parseSlashEventsFromBlock(blockResult.EndBlockEvents)
	}
}

func (m *Monitor) parseSlashEventsFromTxs(txsResults []*abci.ResponseDeliverTx) {
	for _, txResult := range txsResults {
		for _, event := range txResult.Events {
			if m.IsTargetedSlashEvent(event) {
				requestID, _ := getAttributeValue(event, "request_id")
				ErrCnt.WithLabelValues("slashed for request id ", requestID, " due to invalid response")
				common.Logger.Warnf("slashed for request id %s due to invalid response", requestID)
			}
		}
	}
}

func (m *Monitor) parseSlashEventsFromBlock(endBlockEvents []abci.Event) {
	for _, event := range endBlockEvents {
		if m.IsTargetedSlashEvent(event) {
			requestID, _ := getAttributeValue(event, "request_id")
			ErrCnt.WithLabelValues("slashed for request id ", requestID, " due to timeouted")
			common.Logger.Warnf("slashed for request id %s due to response timeouted", requestID)
		}
	}
}

func (m *Monitor) IsTargetedSlashEvent(event abci.Event) bool {
	if event.Type != ServiceSlashingEventType {
		return false
	}

	providerAddr, err := getAttributeValue(event, "provider")
	if err != nil {
		return false
	}

	if _, ok := m.ProviderAddresses[providerAddr]; !ok {
		return false
	}

	return true
}

func (m *Monitor) Stop() {
	common.Logger.Info("onitor stopped")
	m.stopped = true
}

func getAttributeValue(event abci.Event, attributeKey string) (string, error) {
	stringEvents := types.StringifyEvents([]abci.Event{event})
	return stringEvents.GetValue(event.Type, attributeKey)
}