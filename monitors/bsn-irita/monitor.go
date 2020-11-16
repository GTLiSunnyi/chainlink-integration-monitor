package bsnirita

import (
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"

	servicesdk "github.com/irisnet/service-sdk-go"
	"github.com/irisnet/service-sdk-go/types"

	"github.com/smartcontractkit/chainlink/core/logger"

	"github.com/secret2830/chainlink-integration-monitor/base"
)

var _ base.IMonitor = &Monitor{}

const (
	ServiceSlashingEventType = "service_slash"
)

type Monitor struct {
	Client       servicesdk.ServiceClient
	Endpoint     base.Endpoint
	Interval     time.Duration
	ProviderAddr string
	lastHeight   int64
	stopped      bool
}

func NewMonitor(
	endpoint base.Endpoint,
	interval time.Duration,
	providerAddr string,
) *Monitor {
	cfg := types.ClientConfig{
		NodeURI: endpoint.URL,
	}
	serviceClient := servicesdk.NewServiceClient(cfg)

	return &Monitor{
		Client:       serviceClient,
		Endpoint:     endpoint,
		Interval:     interval,
		ProviderAddr: providerAddr,
	}
}

func (m *Monitor) Start() {
	logger.Infof("BSN-IRITA monitor started, provider address: %s", m.ProviderAddr)

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
		logger.Warnf("BSN-IRITA: failed to retrieve the latest block height: %s", err)
		return
	}

	logger.Infof("BSN-IRITA: block height: %d", currentHeight)

	if m.lastHeight == 0 {
		m.lastHeight = currentHeight - 1
	}

	m.scanByRange(m.lastHeight+1, currentHeight)
}

func (m Monitor) getLatestHeight() (int64, error) {
	res, err := m.Client.Status()
	if err != nil {
		return -1, err
	}

	return res.SyncInfo.LatestBlockHeight, nil
}

func (m *Monitor) scanByRange(startHeight int64, endHeight int64) {
	for h := startHeight; h <= endHeight; h++ {
		blockResult, err := m.Client.BlockResults(&h)
		if err != nil {
			logger.Warnf("BSN-IRITA: failed to retrieve the block result, height: %d, err: %s", h, err)
			continue
		}

		m.parseSlashEvents(blockResult)
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
			if m.IsSlashEvent(event) {
				requestID, _ := getAttributeValue(event, "request_id")
				logger.Warnf("BSN-IRITA: slashed for request id %s due to invalid response", requestID)
			}
		}
	}
}

func (m *Monitor) parseSlashEventsFromBlock(endBlockEvents []abci.Event) {
	for _, event := range endBlockEvents {
		if m.IsSlashEvent(event) {
			requestID, _ := getAttributeValue(event, "request_id")
			logger.Warnf("BSN-IRITA: slashed for request id %s due to timeouted", requestID)
		}
	}
}

func (m *Monitor) IsSlashEvent(event abci.Event) bool {
	if event.Type != ServiceSlashingEventType {
		return false
	}

	providerAddr, err := getAttributeValue(event, "provider")
	if err != nil || providerAddr != m.ProviderAddr {
		return false
	}

	return true
}

func (m *Monitor) Stop() {
	logger.Info("BSN-IRITA monitor stopped")
	m.stopped = true
}

func getAttributeValue(event abci.Event, attributeKey string) (string, error) {
	stringEvents := types.StringifyEvents([]abci.Event{event})
	return stringEvents.GetValue(event.Type, attributeKey)
}
