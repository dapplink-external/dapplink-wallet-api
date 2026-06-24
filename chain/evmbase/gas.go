package evmbase

import (
	"math/big"

	"github.com/ethereum/go-ethereum/log"
)

const (
	defaultGasTipCap = 1_000_000_000 // 1 gwei fallback tip
	defaultGasFeeCap = 3_000_000_000 // 3 gwei fallback max fee
)

// ResolveEip1559GasFees loads gas tip/max fee from RPC.
// maxFeePerGas prefers 2*baseFee+tip when base fee is available, otherwise gasPrice with buffer.
func ResolveEip1559GasFees(client EthClient) (gasTipCap, gasFeeCap *big.Int, err error) {
	gasPrice, err := client.SuggestGasPrice()
	if err != nil {
		return nil, nil, err
	}
	if gasPrice.Sign() <= 0 {
		gasPrice = big.NewInt(defaultGasFeeCap)
	}

	gasTipCap, tipErr := client.SuggestGasTipCap()
	if tipErr != nil || gasTipCap == nil || gasTipCap.Sign() <= 0 {
		gasTipCap = new(big.Int).Div(new(big.Int).Set(gasPrice), big.NewInt(10))
		if gasTipCap.Sign() <= 0 {
			gasTipCap = big.NewInt(1)
		}
	}

	header, headerErr := client.BlockHeaderByNumber(nil)
	if headerErr == nil && header != nil && header.BaseFee != nil && header.BaseFee.Sign() > 0 {
		gasFeeCap = new(big.Int).Mul(header.BaseFee, big.NewInt(2))
		gasFeeCap.Add(gasFeeCap, gasTipCap)
	} else {
		gasFeeCap = new(big.Int).Mul(gasPrice, big.NewInt(12))
		gasFeeCap.Div(gasFeeCap, big.NewInt(10))
	}

	if gasFeeCap.Cmp(gasTipCap) < 0 {
		gasFeeCap = new(big.Int).Set(gasTipCap)
	}

	log.Info("resolved eip1559 gas fees",
		"gasTipCap", gasTipCap.String(),
		"gasFeeCap", gasFeeCap.String(),
		"gasPrice", gasPrice.String(),
	)
	return gasTipCap, gasFeeCap, nil
}

func DefaultEip1559GasFees() (gasTipCap, gasFeeCap *big.Int) {
	return big.NewInt(defaultGasTipCap), big.NewInt(defaultGasFeeCap)
}
