package aa

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// paymasterSigPlaceholder marks that a paymaster signature slot will exist on chain.
// UserOpHash must be computed with the paymaster sig magic suffix (see account-abstraction fillAndSign).
var paymasterSigPlaceholder = []byte{0x00}

// BuildTransferUserOp constructs an unsigned sponsored UserOperation for ERC20 transfer.
func BuildTransferUserOp(cfg Config, from, to, token common.Address, amount *big.Int, entryPointNonce *big.Int, authNonce uint64, requestID string, maxFee, maxPriorityFee *big.Int) (*UserOperation, error) {
	callData, err := EncodeExecuteTokenTransferCall(token, to, amount)
	if err != nil {
		return nil, err
	}
	paymasterData := EncodePaymasterSignedData()
	return &UserOperation{
		Sender:               from,
		Nonce:                entryPointNonce,
		InitCode:             common.FromHex(InitCodeEIP7702Marker),
		CallData:             callData,
		VerificationGasLimit: big.NewInt(DefaultVerificationGas),
		CallGasLimit:         big.NewInt(DefaultCallGas),
		PreVerificationGas:   big.NewInt(DefaultPreVerificationGas),
		MaxFeePerGas:         maxFee,
		MaxPriorityFeePerGas: maxPriorityFee,
		Paymaster:            cfg.Paymaster,
		PaymasterData:        paymasterData,
		PaymasterSignature:   paymasterSigPlaceholder,
		EntryPoint:           cfg.EntryPoint,
		ChainID:              cfg.ChainID,
		AuthNonce:            authNonce,
		Delegate:             cfg.Delegate,
		RequestID:            requestID,
	}, nil
}

// FinalizeAndSignSend prepares signed user op, paymaster sig, and SetCode tx.
func FinalizeAndSignSend(cfg Config, op *UserOperation, userOpSig, authSig []byte, sponsorNonce uint64, gasTipCap, gasFeeCap *big.Int, gasLimit uint64) (*types.Transaction, error) {
	EnsurePaymasterSigPlaceholder(op)
	op.Signature = NormalizeECDSAV(userOpSig)
	until, after, err := DecodePaymasterWindow(op.PaymasterData)
	if err != nil {
		return nil, err
	}
	userOpHash := GetUserOpHash(op)

	verifyKey, err := LoadVerifyingKey(cfg)
	if err != nil {
		return nil, err
	}
	pmSig, err := SignPaymaster(userOpHash, until, after, crypto.FromECDSA(verifyKey))
	if err != nil {
		return nil, err
	}
	AttachPaymasterSignature(op, pmSig)

	packed := PackUserOp(op, false)
	handleOpsData, err := EncodeHandleOps(packed, cfg.SponsorAddress)
	if err != nil {
		return nil, err
	}
	auth := AuthFromSignature(cfg.ChainID, cfg.Delegate, op.AuthNonce, authSig)
	return BuildSetCodeTransaction(cfg, sponsorNonce, gasTipCap, gasFeeCap, gasLimit, handleOpsData, auth)
}
