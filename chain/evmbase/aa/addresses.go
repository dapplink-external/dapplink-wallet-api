package aa

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// ParseConfiguredAddress validates a 20-byte hex address from config.
func ParseConfiguredAddress(field, value string) (common.Address, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return common.Address{}, fmt.Errorf("%s is empty", field)
	}
	if !common.IsHexAddress(value) {
		return common.Address{}, fmt.Errorf("invalid %s address %q (expected 0x + 40 hex chars)", field, value)
	}
	return common.HexToAddress(value), nil
}

// ApplyConfigToUserOp overwrites deployment fields from runtime config so a stale
// UserOpJson cannot send the wrong paymaster on chain.
func ApplyConfigToUserOp(op *UserOperation, cfg Config) {
	op.EntryPoint = cfg.EntryPoint
	op.Delegate = cfg.Delegate
	op.Paymaster = cfg.Paymaster
	op.ChainID = cfg.ChainID
}
