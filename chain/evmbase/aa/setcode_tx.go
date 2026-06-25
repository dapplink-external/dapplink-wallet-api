package aa

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

// Config holds AA deployment and sponsor settings.
type Config struct {
	Enabled             bool
	EntryPoint          common.Address
	Delegate            common.Address
	Paymaster           common.Address
	SponsorAddress      common.Address
	SponsorPrivateKey   string
	VerifyingPrivateKey string
	ChainID             *big.Int
}

var ErrMissingSponsorKey = errors.New("sponsor private key not configured; set SPONSOR_KEY or --sponsor-private-key")

func LoadSponsorKey(cfg Config) (*ecdsa.PrivateKey, error) {
	keyHex := strings.TrimPrefix(strings.TrimSpace(cfg.SponsorPrivateKey), "0x")
	if keyHex == "" {
		return nil, ErrMissingSponsorKey
	}
	return crypto.HexToECDSA(keyHex)
}

var ErrMissingPaymasterSignerKey = errors.New("paymaster signer key not configured; set PAYMASTER_SIGNER_KEY or --paymaster-signer-key")

func LoadVerifyingKey(cfg Config) (*ecdsa.PrivateKey, error) {
	keyHex := strings.TrimSpace(cfg.VerifyingPrivateKey)
	if keyHex == "" {
		return nil, ErrMissingPaymasterSignerKey
	}
	keyHex = strings.TrimPrefix(keyHex, "0x")
	return crypto.HexToECDSA(keyHex)
}

// BuildSetCodeTransaction assembles and signs the outer Type-4 transaction.
func BuildSetCodeTransaction(
	cfg Config,
	sponsorNonce uint64,
	gasTipCap, gasFeeCap *big.Int,
	gasLimit uint64,
	handleOpsData []byte,
	auth types.SetCodeAuthorization,
) (*types.Transaction, error) {
	sponsorKey, err := LoadSponsorKey(cfg)
	if err != nil {
		return nil, err
	}

	chainID, _ := uint256.FromBig(cfg.ChainID)
	txData := &types.SetCodeTx{
		ChainID:   chainID,
		Nonce:     sponsorNonce,
		GasTipCap: uint256.MustFromBig(gasTipCap),
		GasFeeCap: uint256.MustFromBig(gasFeeCap),
		Gas:       gasLimit,
		To:        cfg.EntryPoint,
		Value:     uint256.NewInt(0),
		Data:      handleOpsData,
		AuthList:  []types.SetCodeAuthorization{auth},
	}
	signer := types.NewPragueSigner(cfg.ChainID)
	return types.SignNewTx(sponsorKey, signer, txData)
}

// ParseSignatureHex decodes a 65-byte hex signature.
func ParseSignatureHex(sig string) ([]byte, error) {
	sig = strings.TrimPrefix(sig, "0x")
	return hex.DecodeString(sig)
}

// AuthFromSignature builds SetCodeAuthorization from user signature components.
func AuthFromSignature(chainID *big.Int, delegate common.Address, nonce uint64, sig []byte) types.SetCodeAuthorization {
	uChain, _ := uint256.FromBig(chainID)
	auth := types.SetCodeAuthorization{
		ChainID: *uChain,
		Address: delegate,
		Nonce:   nonce,
	}
	if len(sig) >= 65 {
		auth.V = sig[64]
		var r, s uint256.Int
		r.SetBytes(sig[:32])
		s.SetBytes(sig[32:64])
		auth.R = r
		auth.S = s
	}
	return auth
}

// RawTxHex returns 0x-prefixed raw transaction bytes.
func RawTxHex(tx *types.Transaction) (string, error) {
	raw, err := tx.MarshalBinary()
	if err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(raw), nil
}
