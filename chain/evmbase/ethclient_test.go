package evmbase

import (
	"context"
	"testing"
	"time"
	
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/dapplink-labs/dapplink-wallet-api/config"
)

func setup() (ethClient EthClient, ethData *EthData, err error) {

	conf, err := config.NewConfig("../../config.yml", "", "")
	if err != nil {
		log.Error("load config failed, error:", err)
		return nil, ethData, err
	}
	ethClient, err = DialEthClient(context.Background(), conf.WalletNode.Op.RpcUrl)
	if err != nil {
		return nil, ethData, err
	}

	ethDataClient, err := NewEthDataClient(conf.WalletNode.Op.DataApiUrl, conf.WalletNode.Op.DataApiKey, time.Duration(conf.WalletNode.Eth.TimeOut))
	if err != nil {
		return nil, ethData, err
	}

	return ethClient, ethDataClient, nil

}

func TestClnt_EthGetCode(t *testing.T) {
	ethClient, _, err := setup()
	if err != nil {
		t.Error(err)
	}
	code, err := ethClient.EthGetCode(common.HexToAddress("0x48B4bBEbF0655557A461e91B8905b85864B8BB48"))
	if err != nil {
		t.Error(err)
	}
	t.Log(code)
}
