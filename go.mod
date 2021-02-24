module github.com/secret2830/chainlink-integration-monitor

go 1.13

require (
	github.com/irisnet/service-sdk-go v1.0.0-rc1.0.20210222034203-c53ebcbbc1ff
	github.com/prometheus/client_golang v1.8.0
	github.com/sethgrid/pester v1.1.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/viper v1.7.1
	github.com/tendermint/tendermint v0.34.3
)

replace (
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.2-alpha.regen.4
	github.com/tendermint/tendermint => github.com/bianjieai/tendermint v0.34.1-irita-210113
)
