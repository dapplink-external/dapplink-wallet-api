package bnbchain

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"github.com/dapplink-labs/dapplink-wallet-api/chain/evmbase"
)

var (
	transferEventTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
	handleOpsSelector  = crypto.Keccak256([]byte("handleOps((address,uint256,bytes,bytes,bytes32,uint256,bytes32,bytes,bytes)[],address)"))[:4]
)

type userOpERC20Transfer struct {
	Contract string
	From     string
	To       string
	Amount   string
}

func (c *ChainAdaptor) tryParseUserOpERC20Transfer(blockItem evmbase.TransactionList) (userOpERC20Transfer, bool) {
	if c.entryPointAddress == (common.Address{}) {
		return userOpERC20Transfer{}, false
	}
	if normalizeAddress(blockItem.To) != normalizeAddress(c.entryPointAddress.Hex()) {
		return userOpERC20Transfer{}, false
	}
	input := strings.TrimPrefix(blockItem.Input, "0x")
	if len(input) < 8 || !strings.EqualFold(input[:8], common.Bytes2Hex(handleOpsSelector)) {
		return userOpERC20Transfer{}, false
	}

	receipt, err := c.ethClient.TxReceiptByHash(common.HexToHash(blockItem.Hash))
	if err != nil || receipt == nil {
		log.Warn("fetch receipt for UserOp tx failed", "hash", blockItem.Hash, "err", err)
		return userOpERC20Transfer{}, false
	}

	return c.parseUserOpERC20TransferFromReceipt(receipt.Logs)
}

func (c *ChainAdaptor) parseUserOpERC20TransferFromReceipt(logs []*types.Log) (userOpERC20Transfer, bool) {
	for _, lg := range logs {
		if lg == nil || len(lg.Topics) != 3 || lg.Topics[0] != transferEventTopic {
			continue
		}
		token := normalizeAddress(lg.Address.Hex())
		if len(c.contractAddrIndex) > 0 {
			if _, tracked := c.contractAddrIndex[token]; !tracked {
				continue
			}
		}
		from := common.HexToAddress(lg.Topics[1].Hex()).Hex()
		to := common.HexToAddress(lg.Topics[2].Hex()).Hex()
		value := new(big.Int).SetBytes(lg.Data)
		return userOpERC20Transfer{
			Contract: lg.Address.Hex(),
			From:     from,
			To:       to,
			Amount:   value.String(),
		}, true
	}
	return userOpERC20Transfer{}, false
}
