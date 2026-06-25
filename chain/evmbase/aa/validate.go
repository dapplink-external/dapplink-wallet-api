package aa

import (
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

// PreflightReport captures checks run immediately before broadcasting.
type PreflightReport struct {
	UserOpHash            common.Hash
	UserSigRecovered      common.Address
	PaymasterSigRecovered common.Address
	VerifyingSigner       common.Address
	HasPMPlaceholder      bool
	PaymasterDataLen      int
	UserSigLen            int
	Errors                []string
}

func (r PreflightReport) OK() bool { return len(r.Errors) == 0 }

func (r PreflightReport) String() string { return strings.Join(r.Errors, "; ") }

// EnsurePaymasterSigPlaceholder keeps userOpHash stable once paymaster sig is attached on chain.
func EnsurePaymasterSigPlaceholder(op *UserOperation) {
	if op.Paymaster != (common.Address{}) && len(op.PaymasterSignature) == 0 {
		op.PaymasterSignature = paymasterSigPlaceholder
	}
}

// VerifyingSignerAddress derives the configured paymaster verifying signer address.
func VerifyingSignerAddress(cfg Config) (common.Address, error) {
	key, err := LoadVerifyingKey(cfg)
	if err != nil {
		return common.Address{}, err
	}
	return crypto.PubkeyToAddress(key.PublicKey), nil
}

// PreflightSponsoredSend validates user/paymaster signatures before broadcast.
func PreflightSponsoredSend(cfg Config, op *UserOperation, userOpSig []byte, onChainVerifyingSigner common.Address) PreflightReport {
	EnsurePaymasterSigPlaceholder(op)

	report := PreflightReport{
		UserOpHash:       GetUserOpHash(op),
		HasPMPlaceholder: len(op.PaymasterSignature) > 0,
		PaymasterDataLen: len(op.PaymasterData),
		UserSigLen:       len(userOpSig),
	}

	if onChainVerifyingSigner != (common.Address{}) {
		report.VerifyingSigner = onChainVerifyingSigner
	} else if addr, err := VerifyingSignerAddress(cfg); err == nil {
		report.VerifyingSigner = addr
	}

	if !report.HasPMPlaceholder {
		report.Errors = append(report.Errors, "missing paymasterSignature placeholder; userOpHash will not match EntryPoint")
	}
	if report.PaymasterDataLen != 64 {
		report.Errors = append(report.Errors, fmt.Sprintf("paymasterData length=%d want 64", report.PaymasterDataLen))
	}
	if len(userOpSig) != 65 {
		report.Errors = append(report.Errors, fmt.Sprintf("user signature length=%d want 65", len(userOpSig)))
	} else {
		userOpSig = NormalizeECDSAV(userOpSig)
		if pub, err := recoverPubkey(report.UserOpHash.Bytes(), userOpSig); err != nil {
			report.Errors = append(report.Errors, "user signature invalid: "+err.Error())
		} else {
			report.UserSigRecovered = crypto.PubkeyToAddress(*pub)
			if report.UserSigRecovered != op.Sender {
				report.Errors = append(report.Errors, fmt.Sprintf(
					"user signature recovers %s, want sender %s (userOpHash=%s)",
					report.UserSigRecovered.Hex(), op.Sender.Hex(), report.UserOpHash.Hex(),
				))
			}
		}
	}

	until, after, err := DecodePaymasterWindow(op.PaymasterData)
	if err != nil {
		report.Errors = append(report.Errors, "paymaster window: "+err.Error())
		return report
	}

	verifyKey, err := LoadVerifyingKey(cfg)
	if err != nil {
		report.Errors = append(report.Errors, "paymaster signer key: "+err.Error())
		return report
	}
	localSigner := crypto.PubkeyToAddress(verifyKey.PublicKey)
	if report.VerifyingSigner == (common.Address{}) {
		report.VerifyingSigner = localSigner
	} else if localSigner != report.VerifyingSigner {
		report.Errors = append(report.Errors, fmt.Sprintf(
			"local paymaster signer %s != on-chain verifyingSigner %s (check PAYMASTER_SIGNER_KEY)",
			localSigner.Hex(), report.VerifyingSigner.Hex(),
		))
	}

	pmSig, err := SignPaymaster(report.UserOpHash, until, after, crypto.FromECDSA(verifyKey))
	if err != nil {
		report.Errors = append(report.Errors, "sign paymaster: "+err.Error())
		return report
	}
	pmDigest := HashPaymasterData(report.UserOpHash, until, after)
	if pub, err := recoverPubkey(pmDigest, pmSig); err != nil {
		report.Errors = append(report.Errors, "paymaster signature invalid: "+err.Error())
	} else {
		report.PaymasterSigRecovered = crypto.PubkeyToAddress(*pub)
		if report.PaymasterSigRecovered != report.VerifyingSigner {
			report.Errors = append(report.Errors, fmt.Sprintf(
				"paymaster signature recovers %s, want %s",
				report.PaymasterSigRecovered.Hex(), report.VerifyingSigner.Hex(),
			))
		}
	}

	packed := PackUserOp(op, false)
	if len(packed.PaymasterAndData) >= 20 {
		got := common.BytesToAddress(packed.PaymasterAndData[:20])
		if got != cfg.Paymaster {
			report.Errors = append(report.Errors, fmt.Sprintf("packed paymaster %s != config %s", got.Hex(), cfg.Paymaster.Hex()))
		}
	}

	return report
}

func LogPreflight(report PreflightReport, op *UserOperation, cfg Config) {
	log.Info("aa preflight",
		"ok", report.OK(),
		"userOpHash", report.UserOpHash.Hex(),
		"sender", op.Sender.Hex(),
		"authNonce", op.AuthNonce,
		"delegate", op.Delegate.Hex(),
		"entryPoint", op.EntryPoint.Hex(),
		"paymaster", cfg.Paymaster.Hex(),
		"hasPmPlaceholder", report.HasPMPlaceholder,
		"paymasterDataLen", report.PaymasterDataLen,
		"userSigLen", report.UserSigLen,
		"userSigRecovered", report.UserSigRecovered.Hex(),
		"paymasterSigner", report.VerifyingSigner.Hex(),
		"paymasterSigRecovered", report.PaymasterSigRecovered.Hex(),
	)
	if !report.OK() {
		log.Warn("aa preflight failed", "reason", report.String())
	}
}

func recoverPubkey(hash, sig []byte) (*ecdsa.PublicKey, error) {
	if len(sig) != 65 {
		return nil, fmt.Errorf("sig len %d", len(sig))
	}
	s := append([]byte(nil), sig...)
	if s[64] >= 27 {
		s[64] -= 27
	}
	return crypto.SigToPub(hash, s)
}
