package aa

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var paymasterDataArgs = abi.Arguments{
	{Type: mustType("uint256")},
	{Type: mustType("uint256")},
}

// EncodePaymasterSignedData abi.encode(validUntil, validAfter) as uint256 slots.
func EncodePaymasterSignedData() []byte {
	data, _ := encodePaymasterWindow(DefaultValidityWindow())
	return data
}

func DefaultValidityWindow() (uint64, uint64) {
	now := uint64(time.Now().Unix())
	return now + 3600, 0
}

func encodePaymasterWindow(until, after uint64) ([]byte, error) {
	return paymasterDataArgs.Pack(new(big.Int).SetUint64(until), new(big.Int).SetUint64(after))
}

func PackPaymasterDataWithWindow() ([]byte, uint64, uint64) {
	until, after := DefaultValidityWindow()
	encoded, err := encodePaymasterWindow(until, after)
	if err != nil {
		return EncodePaymasterSignedData(), until, after
	}
	return encoded, until, after
}

func DecodePaymasterWindow(data []byte) (uint64, uint64, error) {
	if len(data) == 0 {
		until, after := DefaultValidityWindow()
		return until, after, nil
	}
	values, err := paymasterDataArgs.Unpack(data)
	if err != nil {
		return 0, 0, err
	}
	until, ok := values[0].(*big.Int)
	if !ok {
		return 0, 0, fmt.Errorf("invalid paymaster validUntil")
	}
	after, ok := values[1].(*big.Int)
	if !ok {
		return 0, 0, fmt.Errorf("invalid paymaster validAfter")
	}
	return until.Uint64(), after.Uint64(), nil
}

// HashPaymasterData matches SponsorPaymaster keccak256(abi.encode(userOpHash, validUntil, validAfter)).
func HashPaymasterData(userOpHash common.Hash, validUntil, validAfter uint64) []byte {
	args := abi.Arguments{
		{Type: mustType("bytes32")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
	}
	encoded, _ := args.Pack(userOpHash, new(big.Int).SetUint64(validUntil), new(big.Int).SetUint64(validAfter))
	return crypto.Keccak256(encoded)
}

func SignPaymaster(userOpHash common.Hash, validUntil, validAfter uint64, signerKey []byte) ([]byte, error) {
	data := HashPaymasterData(userOpHash, validUntil, validAfter)
	priv, err := crypto.ToECDSA(signerKey)
	if err != nil {
		return nil, err
	}
	sig, err := crypto.Sign(data, priv)
	if err != nil {
		return nil, err
	}
	return NormalizeECDSAV(sig), nil
}

func AttachPaymasterSignature(op *UserOperation, pmSig []byte) {
	op.PaymasterSignature = pmSig
}

func BigIntChainID(chainID string) *big.Int {
	id, ok := new(big.Int).SetString(chainID, 10)
	if !ok {
		return big.NewInt(56)
	}
	return id
}
