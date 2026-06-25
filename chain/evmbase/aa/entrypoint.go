package aa

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

var entryPointABI = mustParseABI(`[
  {"inputs":[{"components":[{"internalType":"address","name":"sender","type":"address"},{"internalType":"uint256","name":"nonce","type":"uint256"},{"internalType":"bytes","name":"initCode","type":"bytes"},{"internalType":"bytes","name":"callData","type":"bytes"},{"internalType":"bytes32","name":"accountGasLimits","type":"bytes32"},{"internalType":"uint256","name":"preVerificationGas","type":"uint256"},{"internalType":"bytes32","name":"gasFees","type":"bytes32"},{"internalType":"bytes","name":"paymasterAndData","type":"bytes"},{"internalType":"bytes","name":"signature","type":"bytes"}],"internalType":"struct PackedUserOperation[]","name":"ops","type":"tuple[]"},{"internalType":"address payable","name":"beneficiary","type":"address"}],"name":"handleOps","outputs":[],"stateMutability":"nonpayable","type":"function"},
  {"inputs":[{"internalType":"address","name":"sender","type":"address"},{"internalType":"uint192","name":"key","type":"uint192"}],"name":"getNonce","outputs":[{"internalType":"uint256","name":"nonce","type":"uint256"}],"stateMutability":"view","type":"function"}
]`)

var paymasterViewABI = mustParseABI(`[
  {"inputs":[],"name":"verifyingSigner","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"}
]`)

func mustParseABI(raw string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(raw))
	if err != nil {
		panic(err)
	}
	return parsed
}

type packedUserOpTuple struct {
	Sender             common.Address
	Nonce              *big.Int
	InitCode           []byte
	CallData           []byte
	AccountGasLimits   [32]byte
	PreVerificationGas *big.Int
	GasFees            [32]byte
	PaymasterAndData   []byte
	Signature          []byte
}

func EncodeHandleOps(op PackedUserOperation, beneficiary common.Address) ([]byte, error) {
	tuple := packedUserOpTuple{
		Sender:             op.Sender,
		Nonce:              op.Nonce,
		InitCode:           op.InitCode,
		CallData:           op.CallData,
		AccountGasLimits:   op.AccountGasLimits,
		PreVerificationGas: op.PreVerificationGas,
		GasFees:            op.GasFees,
		PaymasterAndData:   op.PaymasterAndData,
		Signature:          op.Signature,
	}
	return entryPointABI.Pack("handleOps", []packedUserOpTuple{tuple}, beneficiary)
}

func EncodeGetNonce(sender common.Address, key *big.Int) ([]byte, error) {
	return entryPointABI.Pack("getNonce", sender, key)
}

func EncodePaymasterVerifyingSigner() ([]byte, error) {
	return paymasterViewABI.Pack("verifyingSigner")
}

// HandleOpsSelector returns the 4-byte method id for handleOps.
func HandleOpsSelector() []byte {
	return entryPointABI.Methods["handleOps"].ID
}
