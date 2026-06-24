package solana

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mr-tron/base58"

	"github.com/dapplink-labs/dapplink-wallet-api/chain"
	"github.com/dapplink-labs/dapplink-wallet-api/config"
	"github.com/dapplink-labs/dapplink-wallet-api/protobuf/common"
	"github.com/dapplink-labs/dapplink-wallet-api/protobuf/walletapi"
)

const (
	ChainID string = "DappLinkSolana"
)

type ChainAdaptor struct {
	solCli    SolClient
	sdkClient *rpc.Client
	solData   *SolData
}

func NewChainAdaptor(conf *config.Config) (chain.IChainAdaptor, error) {
	rpcUrl := conf.WalletNode.Sol.RpcUrl

	solHttpCli, err := NewSolHttpClient(rpcUrl)
	if err != nil {
		log.Error("Dial solana client fail", "err", err)
		return nil, err
	}
	dataApiUrl := conf.WalletNode.Sol.DataApiUrl
	dataApiKey := conf.WalletNode.Sol.DataApiKey
	dataApiTimeOut := conf.WalletNode.Sol.TimeOut
	solData, err := NewSolScanClient(dataApiUrl, dataApiKey, time.Duration(dataApiTimeOut))
	if err != nil {
		log.Error("new solana data client fail", "err", err)
		return nil, err
	}

	sdkClient := rpc.New(rpcUrl)

	return &ChainAdaptor{
		solCli:    solHttpCli,
		sdkClient: sdkClient,
		solData:   solData,
	}, nil
}

func (c *ChainAdaptor) ConvertAddresses(ctx context.Context, req *walletapi.ConvertAddressesRequest) (*walletapi.ConvertAddressesResponse, error) {
	var retAddressList []*walletapi.Addresses
	for _, publicKeyItem := range req.PublicKey {
		var addressItem *walletapi.Addresses
		publicKeyBytes, err := hex.DecodeString(strings.TrimPrefix(publicKeyItem.PublicKey, "0x"))
		if err != nil {
			addressItem = &walletapi.Addresses{
				Address:   "",
				PublicKey: publicKeyItem.PublicKey,
				Type:      publicKeyItem.Type,
			}
			log.Error("decode public key fail", "err", err)
		} else {
			pubKey := solana.PublicKeyFromBytes(publicKeyBytes)
			log.Info("convert addresses", "address", pubKey.String())
			addressItem = &walletapi.Addresses{
				Address:   pubKey.String(),
				PublicKey: publicKeyItem.PublicKey,
				Type:      publicKeyItem.Type,
			}
		}
		retAddressList = append(retAddressList, addressItem)
	}
	return &walletapi.ConvertAddressesResponse{
		Code:    common.ReturnCode_SUCCESS,
		Msg:     "success",
		Address: retAddressList,
	}, nil
}

func (c *ChainAdaptor) ValidAddresses(ctx context.Context, req *walletapi.ValidAddressesRequest) (*walletapi.ValidAddressesResponse, error) {
	var retAddressList []*walletapi.AddressesValid
	for _, address := range req.Addresses {
		_, err := solana.PublicKeyFromBase58(address.Address)
		if err != nil {
			retAddressList = append(retAddressList, &walletapi.AddressesValid{
				Address: address.Address,
				Valid:   false,
			})
		} else {
			retAddressList = append(retAddressList, &walletapi.AddressesValid{
				Address: address.Address,
				Valid:   true,
			})
		}
	}
	return &walletapi.ValidAddressesResponse{
		Code:         common.ReturnCode_SUCCESS,
		Msg:          "success",
		AddressValid: retAddressList,
	}, nil
}

func (c *ChainAdaptor) GetLastestBlock(ctx context.Context, req *walletapi.LastestBlockRequest) (*walletapi.LastestBlockResponse, error) {
	slot, err := c.solCli.GetSlot(Finalized)
	if err != nil {
		log.Error("get latest slot fail", "err", err)
		return &walletapi.LastestBlockResponse{
			Code: common.ReturnCode_ERROR,
			Msg:  err.Error(),
		}, err
	}

	return &walletapi.LastestBlockResponse{
		Code:   common.ReturnCode_SUCCESS,
		Msg:    "success",
		Height: slot,
	}, nil
}

func (c *ChainAdaptor) GetBlock(ctx context.Context, req *walletapi.BlockRequest) (*walletapi.BlockResponse, error) {
	slot, err := strconv.ParseUint(req.HashHeight, 10, 64)
	if err != nil {
		log.Error("parse slot fail", "err", err)
		return &walletapi.BlockResponse{
			Code: common.ReturnCode_ERROR,
			Msg:  err.Error(),
		}, err
	}

	blockResult, err := c.solCli.GetBlockBySlot(slot, Full)
	if err != nil {
		log.Error("get block by slot fail", "err", err)
		return &walletapi.BlockResponse{
			Code: common.ReturnCode_ERROR,
			Msg:  err.Error(),
		}, err
	}

	var txList []*walletapi.TransactionList
	if blockResult.Transactions != nil {
		for _, tx := range blockResult.Transactions {
			txList = append(txList, &walletapi.TransactionList{
				TxHash: tx.Signature,
			})
		}
	}

	return &walletapi.BlockResponse{
		Code:         common.ReturnCode_SUCCESS,
		Msg:          "success",
		Height:       strconv.FormatUint(slot, 10),
		Hash:         blockResult.BlockHash,
		Transactions: txList,
	}, nil
}

func (c *ChainAdaptor) GetBatchBlock(ctx context.Context, req *walletapi.BatchBlockRequest) (*walletapi.BatchBlockResponse, error) {
	return &walletapi.BatchBlockResponse{
		Code: common.ReturnCode_ERROR,
		Msg:  "batch block query is not supported",
	}, nil
}

func (c *ChainAdaptor) GetTransactionByHash(ctx context.Context, req *walletapi.TransactionByHashRequest) (*walletapi.TransactionByHashResponse, error) {
	txResult, err := c.solCli.GetTransaction(req.Hash)
	if err != nil {
		log.Error("get transaction fail", "err", err)
		return &walletapi.TransactionByHashResponse{
			Code: common.ReturnCode_ERROR,
			Msg:  err.Error(),
		}, err
	}

	var fromAddr, toAddr, amount, contractAddress string
	var txType uint32

	if len(txResult.Transaction.Message.Instructions) > 0 {
		instruction := txResult.Transaction.Message.Instructions[0]
		accounts := txResult.Transaction.Message.AccountKeys

		if instruction.ProgramIdIndex < len(accounts) {
			programId := accounts[instruction.ProgramIdIndex]

			if programId == system.ProgramID.String() {
				txType = 1 // Native SOL transfer
				if len(instruction.Accounts) >= 2 {
					fromAddr = accounts[instruction.Accounts[0]]
					toAddr = accounts[instruction.Accounts[1]]
				}
				if len(instruction.Data) >= 4 {
					data, _ := base58.Decode(instruction.Data)
					if len(data) >= 12 {
						lamports := uint64(data[4]) | uint64(data[5])<<8 | uint64(data[6])<<16 | uint64(data[7])<<24 |
							uint64(data[8])<<32 | uint64(data[9])<<40 | uint64(data[10])<<48 | uint64(data[11])<<56
						amount = strconv.FormatUint(lamports, 10)
					}
				}
			} else if programId == token.ProgramID.String() {
				txType = 2 // SPL Token transfer
				contractAddress = programId
				if len(instruction.Accounts) >= 3 {
					fromAddr = accounts[instruction.Accounts[0]]
					toAddr = accounts[instruction.Accounts[1]]
				}
			}
		}
	}

	return &walletapi.TransactionByHashResponse{
		Code: common.ReturnCode_SUCCESS,
		Msg:  "success",
		Transaction: &walletapi.TransactionList{
			TxHash:          req.Hash,
			Fee:             strconv.FormatUint(txResult.Meta.Fee, 10),
			Status:          0,
			TxType:          txType,
			ContractAddress: contractAddress,
			From: []*walletapi.FromAddress{
				{Address: fromAddr, Amount: amount},
			},
			To: []*walletapi.ToAddress{
				{Address: toAddr, Amount: amount},
			},
		},
	}, nil
}

func (c *ChainAdaptor) GetTransactionByAddress(ctx context.Context, req *walletapi.TransactionByAddressRequest) (*walletapi.TransactionByAddressResponse, error) {
	pageSize := req.PageSize
	if pageSize == 0 {
		pageSize = 10
	}

	signatures, err := c.solCli.GetTxForAddress(req.Address, Finalized, pageSize, "", "")
	if err != nil {
		log.Error("get transactions for address fail", "err", err)
		return &walletapi.TransactionByAddressResponse{
			Code: common.ReturnCode_ERROR,
			Msg:  err.Error(),
		}, err
	}

	var txList []*walletapi.TransactionList
	for _, sig := range signatures {
		txList = append(txList, &walletapi.TransactionList{
			TxHash: sig.Signature,
		})
	}

	return &walletapi.TransactionByAddressResponse{
		Code:        common.ReturnCode_SUCCESS,
		Msg:         "success",
		Transaction: txList,
	}, nil
}

func (c *ChainAdaptor) GetAccountBalance(ctx context.Context, req *walletapi.AccountBalanceRequest) (*walletapi.AccountBalanceResponse, error) {
	if req.ContractAddress == "" || req.ContractAddress == "So11111111111111111111111111111111111111112" {
		balance, err := c.solCli.GetBalance(req.Address)
		if err != nil {
			log.Error("get balance fail", "err", err)
			return &walletapi.AccountBalanceResponse{
				Code: common.ReturnCode_ERROR,
				Msg:  err.Error(),
			}, err
		}

		return &walletapi.AccountBalanceResponse{
			Code:    common.ReturnCode_SUCCESS,
			Msg:     "success",
			Balance: strconv.FormatUint(balance, 10),
		}, nil
	} else {
		ownerPubkey, err := solana.PublicKeyFromBase58(req.Address)
		if err != nil {
			log.Error("invalid address", "err", err)
			return &walletapi.AccountBalanceResponse{
				Code: common.ReturnCode_ERROR,
				Msg:  err.Error(),
			}, err
		}

		mintPubkey, err := solana.PublicKeyFromBase58(req.ContractAddress)
		if err != nil {
			log.Error("invalid contract address", "err", err)
			return &walletapi.AccountBalanceResponse{
				Code: common.ReturnCode_ERROR,
				Msg:  err.Error(),
			}, err
		}

		ata, _, err := solana.FindAssociatedTokenAddress(ownerPubkey, mintPubkey)
		if err != nil {
			log.Error("find associated token address fail", "err", err)
			return &walletapi.AccountBalanceResponse{
				Code: common.ReturnCode_ERROR,
				Msg:  err.Error(),
			}, err
		}

		accountInfo, err := GetAccountInfo(c.sdkClient, ata)
		if err != nil {
			return &walletapi.AccountBalanceResponse{
				Code:    common.ReturnCode_SUCCESS,
				Msg:     "success",
				Balance: "0",
			}, nil
		}

		var tokenAccount token.Account
		decoder := bin.NewBinDecoder(accountInfo.GetBinary())
		err = tokenAccount.UnmarshalWithDecoder(decoder)
		if err != nil {
			log.Error("unmarshal token account fail", "err", err)
			return &walletapi.AccountBalanceResponse{
				Code: common.ReturnCode_ERROR,
				Msg:  err.Error(),
			}, err
		}

		return &walletapi.AccountBalanceResponse{
			Code:    common.ReturnCode_SUCCESS,
			Msg:     "success",
			Balance: strconv.FormatUint(tokenAccount.Amount, 10),
		}, nil
	}
}

func (c *ChainAdaptor) SendTransaction(ctx context.Context, req *walletapi.SendTransactionsRequest) (*walletapi.SendTransactionResponse, error) {
	var txnRetList []*walletapi.RawTransactionReturn

	for _, rawTx := range req.RawTx {
		config := &SendTransactionRequest{
			Encoding:            "base64",
			SkipPreflight:       false,
			PreflightCommitment: string(Finalized),
			MaxRetries:          3,
			MinContextSlot:      0,
		}

		txHash, err := c.solCli.SendTransaction(rawTx.RawTx, config)
		if err != nil {
			log.Error("send transaction fail", "err", err)
			txnRetList = append(txnRetList, &walletapi.RawTransactionReturn{
				TxHash:    "",
				IsSuccess: false,
				Message:   err.Error(),
			})
		} else {
			txnRetList = append(txnRetList, &walletapi.RawTransactionReturn{
				TxHash:    txHash,
				IsSuccess: true,
				Message:   "success",
			})
		}
	}

	return &walletapi.SendTransactionResponse{
		Code:   common.ReturnCode_SUCCESS,
		Msg:    "success",
		TxnRet: txnRetList,
	}, nil
}

func (c *ChainAdaptor) BuildTransactionSchema(ctx context.Context, request *walletapi.TransactionSchemaRequest) (*walletapi.TransactionSchemaResponse, error) {
	return &walletapi.TransactionSchemaResponse{
		Code: common.ReturnCode_SUCCESS,
		Msg:  "success",
	}, nil
}

func (c *ChainAdaptor) BuildUnSignTransaction(ctx context.Context, request *walletapi.UnSignTransactionRequest) (*walletapi.UnSignTransactionResponse, error) {
	var unsignedTxList []*walletapi.UnsignedTransactionMessageHash

	for _, base64Txn := range request.Base64Txn {
		// For now, just return the transaction as-is
		// In a real implementation, you would parse the transaction data
		// and build the unsigned transaction
		unsignedTxList = append(unsignedTxList, &walletapi.UnsignedTransactionMessageHash{
			UnsignedTx: base64Txn.Base64Tx,
		})
	}

	return &walletapi.UnSignTransactionResponse{
		Code:        common.ReturnCode_SUCCESS,
		Msg:         "success",
		UnsignedTxn: unsignedTxList,
	}, nil
}

func (c *ChainAdaptor) BuildSignedTransaction(ctx context.Context, request *walletapi.SignedTransactionRequest) (*walletapi.SignedTransactionResponse, error) {
	var signedTxList []*walletapi.SignedTxWithHash

	for _, txnWithSig := range request.TxnWithSignature {
		// Decode unsigned transaction
		txBytes, err := base64.StdEncoding.DecodeString(txnWithSig.Base64Tx)
		if err != nil {
			log.Error("decode base64 tx fail", "err", err)
			signedTxList = append(signedTxList, &walletapi.SignedTxWithHash{
				SignedTx:  "",
				TxHash:    "",
				IsSuccess: false,
			})
			continue
		}

		// Decode signature
		signatureBytes, err := hex.DecodeString(strings.TrimPrefix(txnWithSig.Signature, "0x"))
		if err != nil {
			log.Error("decode signature fail", "err", err)
			signedTxList = append(signedTxList, &walletapi.SignedTxWithHash{
				SignedTx:  "",
				TxHash:    "",
				IsSuccess: false,
			})
			continue
		}

		// Reconstruct transaction with signature
		var message solana.Message
		decoder := bin.NewBinDecoder(txBytes)
		err = message.UnmarshalWithDecoder(decoder)
		if err != nil {
			log.Error("unmarshal message fail", "err", err)
			signedTxList = append(signedTxList, &walletapi.SignedTxWithHash{
				SignedTx:  "",
				TxHash:    "",
				IsSuccess: false,
			})
			continue
		}

		tx := &solana.Transaction{
			Signatures: []solana.Signature{solana.SignatureFromBytes(signatureBytes)},
			Message:    message,
		}

		// Serialize signed transaction
		signedTxBytes, err := tx.MarshalBinary()
		if err != nil {
			log.Error("marshal signed tx fail", "err", err)
			signedTxList = append(signedTxList, &walletapi.SignedTxWithHash{
				SignedTx:  "",
				TxHash:    "",
				IsSuccess: false,
			})
			continue
		}

		// Calculate transaction hash (first signature)
		txHash := base58.Encode(signatureBytes)

		signedTxList = append(signedTxList, &walletapi.SignedTxWithHash{
			SignedTx:  base64.StdEncoding.EncodeToString(signedTxBytes),
			TxHash:    txHash,
			IsSuccess: true,
		})
	}

	return &walletapi.SignedTransactionResponse{
		Code:      common.ReturnCode_SUCCESS,
		Msg:       "success",
		SignedTxn: signedTxList,
	}, nil
}

func (c *ChainAdaptor) GetAddressApproveList(ctx context.Context, request *walletapi.AddressApproveListRequest) (*walletapi.AddressApproveListResponse, error) {
	return &walletapi.AddressApproveListResponse{
		Code: common.ReturnCode_SUCCESS,
		Msg:  "don't support in this stage, support in the future",
	}, nil
}

func PubKeyHexToAddress(pubKeyHex string) (string, error) {
	pubKeyHex = strings.TrimPrefix(pubKeyHex, "0x")
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return "", err
	}
	pubKey := solana.PublicKeyFromBytes(pubKeyBytes)
	return pubKey.String(), nil
}

func isSolTransfer(coinAddress string) bool {
	return coinAddress == "" ||
		coinAddress == "So11111111111111111111111111111111111111112"
}

func getPrivateKey(keyStr string) (solana.PrivateKey, error) {
	if prikey, err := solana.PrivateKeyFromBase58(keyStr); err == nil {
		return prikey, nil
	}
	if bytes, err := hex.DecodeString(keyStr); err == nil {
		return solana.PrivateKey(bytes), nil
	}
	return nil, fmt.Errorf("invalid private key format")
}
