package bnbchain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/holiman/uint256"

	"github.com/dapplink-labs/dapplink-wallet-api/chain/evmbase"
	"github.com/dapplink-labs/dapplink-wallet-api/chain/evmbase/aa"
	common2 "github.com/dapplink-labs/dapplink-wallet-api/protobuf/common"
	"github.com/dapplink-labs/dapplink-wallet-api/protobuf/walletapi"
)

func (c *ChainAdaptor) aaConfig() (aa.Config, error) {
	aaConf := c.conf.WalletNode.BNB.AA
	chainID := aa.BigIntChainID(aaConf.ChainIDNumeric)
	if aaConf.ChainIDNumeric == "" {
		chainID = big.NewInt(56)
	}
	entryPoint, err := aa.ParseConfiguredAddress("entry_point", aaConf.EntryPoint)
	if err != nil {
		return aa.Config{}, err
	}
	delegate, err := aa.ParseConfiguredAddress("delegate", aaConf.Delegate)
	if err != nil {
		return aa.Config{}, err
	}
	paymaster, err := aa.ParseConfiguredAddress("paymaster", aaConf.Paymaster)
	if err != nil {
		return aa.Config{}, err
	}
	sponsorAddress, err := aa.ParseConfiguredAddress("sponsor_address", aaConf.SponsorAddress)
	if err != nil {
		return aa.Config{}, err
	}
	return aa.Config{
		Enabled:             aaConf.Enabled,
		EntryPoint:          entryPoint,
		Delegate:            delegate,
		Paymaster:           paymaster,
		SponsorAddress:      sponsorAddress,
		SponsorPrivateKey:   aaConf.SponsorPrivateKey,
		VerifyingPrivateKey: aaConf.VerifyingPrivateKey,
		ChainID:             chainID,
	}, nil
}

func (c *ChainAdaptor) BuildSponsoredTransfer(ctx context.Context, request *walletapi.SponsoredTransferRequest) (*walletapi.SponsoredTransferBuildResponse, error) {
	cfg, err := c.aaConfig()
	if err != nil {
		return &walletapi.SponsoredTransferBuildResponse{Code: common2.ReturnCode_ERROR, Msg: err.Error()}, nil
	}
	if !cfg.Enabled {
		return &walletapi.SponsoredTransferBuildResponse{Code: common2.ReturnCode_ERROR, Msg: "AA not enabled"}, nil
	}

	from := common.HexToAddress(request.FromAddress)
	to := common.HexToAddress(request.ToAddress)
	token := common.HexToAddress(request.TokenAddress)
	amount, ok := new(big.Int).SetString(request.Amount, 10)
	if !ok {
		return &walletapi.SponsoredTransferBuildResponse{Code: common2.ReturnCode_ERROR, Msg: "invalid amount"}, nil
	}

	authNonceBig, err := c.ethClient.GetTransactionAccount(from)
	if err != nil {
		return &walletapi.SponsoredTransferBuildResponse{Code: common2.ReturnCode_ERROR, Msg: err.Error()}, nil
	}
	authNonce := authNonceBig.Uint64()

	entryNonce, err := c.entryPointNonce(from, cfg.EntryPoint)
	if err != nil {
		return &walletapi.SponsoredTransferBuildResponse{Code: common2.ReturnCode_ERROR, Msg: err.Error()}, nil
	}

	gasTipCap, gasFeeCap, err := evmbase.ResolveEip1559GasFees(c.ethClient)
	if err != nil {
		gasTipCap, gasFeeCap = evmbase.DefaultEip1559GasFees()
	}

	op, err := aa.BuildTransferUserOp(cfg, from, to, token, amount, entryNonce, authNonce, request.RequestId, gasFeeCap, gasTipCap)
	if err != nil {
		return &walletapi.SponsoredTransferBuildResponse{Code: common2.ReturnCode_ERROR, Msg: err.Error()}, nil
	}

	userOpHash := aa.GetUserOpHash(op)
	uChain, _ := uint256.FromBig(cfg.ChainID)
	authDigest := aa.AuthDigest(uChain, cfg.Delegate, authNonce)

	if signer, err := aa.VerifyingSignerAddress(cfg); err == nil {
		log.Info("build sponsored transfer", "from", from.Hex(), "paymaster", cfg.Paymaster.Hex(), "paymasterSigner", signer.Hex(), "authNonce", authNonce, "userOpHash", userOpHash.Hex(), "pmPlaceholder", len(op.PaymasterSignature) > 0)
	} else {
		log.Info("build sponsored transfer", "from", from.Hex(), "paymaster", cfg.Paymaster.Hex(), "authNonce", authNonce, "userOpHash", userOpHash.Hex(), "pmPlaceholder", len(op.PaymasterSignature) > 0)
	}

	opJSON, err := aa.UserOpToJSON(op)
	if err != nil {
		return &walletapi.SponsoredTransferBuildResponse{Code: common2.ReturnCode_ERROR, Msg: err.Error()}, nil
	}

	return &walletapi.SponsoredTransferBuildResponse{
		Code:       common2.ReturnCode_SUCCESS,
		Msg:        "success",
		UserOpHash: userOpHash.Hex(),
		AuthDigest: authDigest.Hex(),
		UserOpJson: opJSON,
	}, nil
}

func (c *ChainAdaptor) SendSponsoredTransfer(ctx context.Context, request *walletapi.SponsoredTransferSendRequest) (*walletapi.SendTransactionResponse, error) {
	cfg, err := c.aaConfig()
	if err != nil {
		return &walletapi.SendTransactionResponse{Code: common2.ReturnCode_ERROR, Msg: err.Error()}, nil
	}
	if !cfg.Enabled {
		return &walletapi.SendTransactionResponse{Code: common2.ReturnCode_ERROR, Msg: "AA not enabled"}, nil
	}

	op, err := aa.UserOpFromJSON(request.UserOpJson)
	if err != nil {
		return &walletapi.SendTransactionResponse{Code: common2.ReturnCode_ERROR, Msg: err.Error()}, nil
	}
	aa.ApplyConfigToUserOp(op, cfg)
	aa.EnsurePaymasterSigPlaceholder(op)

	userOpSig, err := aa.ParseSignatureHex(request.UserOpSignature)
	if err != nil {
		return &walletapi.SendTransactionResponse{Code: common2.ReturnCode_ERROR, Msg: err.Error()}, nil
	}
	authSig, err := aa.ParseSignatureHex(request.AuthSignature)
	if err != nil {
		return &walletapi.SendTransactionResponse{Code: common2.ReturnCode_ERROR, Msg: err.Error()}, nil
	}

	onChainPMSigner, pmErr := c.paymasterVerifyingSigner(cfg.Paymaster)
	if pmErr != nil {
		log.Warn("read paymaster verifyingSigner failed", "err", pmErr)
	}
	report := aa.PreflightSponsoredSend(cfg, op, userOpSig, onChainPMSigner)
	aa.LogPreflight(report, op, cfg)
	if !report.OK() {
		return &walletapi.SendTransactionResponse{Code: common2.ReturnCode_ERROR, Msg: report.String()}, nil
	}

	gasTipCap, gasFeeCap, err := evmbase.ResolveEip1559GasFees(c.ethClient)
	if err != nil {
		gasTipCap, gasFeeCap = evmbase.DefaultEip1559GasFees()
	}

	sponsorNonceBig, err := c.ethClient.GetTransactionAccount(cfg.SponsorAddress)
	if err != nil {
		return &walletapi.SendTransactionResponse{Code: common2.ReturnCode_ERROR, Msg: err.Error()}, nil
	}

	const outerGasLimit = uint64(1_500_000)
	tx, err := aa.FinalizeAndSignSend(cfg, op, userOpSig, authSig, sponsorNonceBig.Uint64(), gasTipCap, gasFeeCap, outerGasLimit)
	if err != nil {
		return &walletapi.SendTransactionResponse{Code: common2.ReturnCode_ERROR, Msg: err.Error()}, nil
	}

	rawTx, err := aa.RawTxHex(tx)
	if err != nil {
		return &walletapi.SendTransactionResponse{Code: common2.ReturnCode_ERROR, Msg: err.Error()}, nil
	}

	hash, err := c.ethClient.SendRawTransaction(rawTx)
	if err != nil {
		return &walletapi.SendTransactionResponse{Code: common2.ReturnCode_ERROR, Msg: err.Error()}, nil
	}

	return &walletapi.SendTransactionResponse{
		Code: common2.ReturnCode_SUCCESS,
		Msg:  "success",
		TxnRet: []*walletapi.RawTransactionReturn{{
			TxHash:    hash.Hex(),
			IsSuccess: true,
			Message:   "sponsored transfer sent",
		}},
	}, nil
}

func (c *ChainAdaptor) entryPointNonce(sender, entryPoint common.Address) (*big.Int, error) {
	data, err := aa.EncodeGetNonce(sender, big.NewInt(0))
	if err != nil {
		return nil, err
	}
	out, err := c.ethClient.CallContract(ethereum.CallMsg{To: &entryPoint, Data: data})
	if err != nil {
		return nil, err
	}
	if len(out) < 32 {
		return nil, fmt.Errorf("invalid getNonce response")
	}
	return new(big.Int).SetBytes(out[len(out)-32:]), nil
}

func (c *ChainAdaptor) paymasterVerifyingSigner(paymaster common.Address) (common.Address, error) {
	data, err := aa.EncodePaymasterVerifyingSigner()
	if err != nil {
		return common.Address{}, err
	}
	out, err := c.ethClient.CallContract(ethereum.CallMsg{To: &paymaster, Data: data})
	if err != nil {
		return common.Address{}, err
	}
	if len(out) < 32 {
		return common.Address{}, fmt.Errorf("invalid verifyingSigner response")
	}
	return common.BytesToAddress(out[len(out)-20:]), nil
}
