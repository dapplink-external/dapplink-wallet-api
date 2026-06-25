package aa

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

// AuthDigest returns the EIP-7702 authorization hash for the user EOA to sign.
func AuthDigest(chainID *uint256.Int, delegate common.Address, nonce uint64) common.Hash {
	auth := types.SetCodeAuthorization{
		ChainID: *chainID,
		Address: delegate,
		Nonce:   nonce,
	}
	return auth.SigHash()
}

// ParseAuthSignature parses hex signature into SetCodeAuthorization fields.
func ParseAuthSignature(chainID *uint256.Int, delegate common.Address, nonce uint64, sigHex []byte) (types.SetCodeAuthorization, error) {
	auth := types.SetCodeAuthorization{
		ChainID: *chainID,
		Address: delegate,
		Nonce:   nonce,
	}
	if len(sigHex) == 65 {
		auth.V = sigHex[64]
		var r, s uint256.Int
		r.SetBytes(sigHex[:32])
		s.SetBytes(sigHex[32:64])
		auth.R = r
		auth.S = s
	}
	return auth, nil
}
