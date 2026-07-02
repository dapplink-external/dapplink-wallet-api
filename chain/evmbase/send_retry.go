package evmbase

import (
	"math/big"
	"strings"
)

// IsRetryableSendError reports nonce/gas conflicts that can be retried with fresh nonce or higher fees.
func IsRetryableSendError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "replacement transaction underpriced") ||
		strings.Contains(msg, "nonce too low") ||
		strings.Contains(msg, "already known")
}

// BumpEip1559GasFees increases tip and max fee for resubmission attempts.
func BumpEip1559GasFees(gasTipCap, gasFeeCap *big.Int, attempt int) (*big.Int, *big.Int) {
	multiplier := big.NewInt(int64(120 + attempt*20)) // 120%, 140%, 160% ...
	tip := new(big.Int).Mul(gasTipCap, multiplier)
	tip.Div(tip, big.NewInt(100))
	fee := new(big.Int).Mul(gasFeeCap, multiplier)
	fee.Div(fee, big.NewInt(100))
	if tip.Sign() <= 0 {
		tip = big.NewInt(1)
	}
	if fee.Cmp(tip) < 0 {
		fee = new(big.Int).Set(tip)
	}
	return tip, fee
}
