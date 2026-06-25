package aa

import (
	"encoding/binary"
	"encoding/json"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	InitCodeEIP7702Marker       = "0x7702"
	PaymasterSigMagic           = "22e325a297439656"
	DomainName                  = "ERC4337"
	DomainVersion               = "1"
	DefaultVerificationGas    = 150_000
	DefaultCallGas            = 120_000
	DefaultPreVerificationGas = 50_000
	DefaultPaymasterVerifyGas = 100_000
	DefaultPaymasterPostOpGas = 50_000
)

var (
	packedUserOpTypeHash = crypto.Keccak256Hash([]byte("PackedUserOperation(address sender,uint256 nonce,bytes initCode,bytes callData,bytes32 accountGasLimits,uint256 preVerificationGas,bytes32 gasFees,bytes paymasterAndData)"))
	eip712DomainTypeHash = crypto.Keccak256Hash([]byte("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"))
)

// UserOperation is the unpacked user operation used across services.
type UserOperation struct {
	Sender               common.Address `json:"sender"`
	Nonce                *big.Int       `json:"nonce"`
	InitCode             []byte         `json:"initCode"`
	CallData             []byte         `json:"callData"`
	VerificationGasLimit *big.Int       `json:"verificationGasLimit"`
	CallGasLimit         *big.Int       `json:"callGasLimit"`
	PreVerificationGas   *big.Int       `json:"preVerificationGas"`
	MaxFeePerGas         *big.Int       `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *big.Int       `json:"maxPriorityFeePerGas"`
	Paymaster            common.Address `json:"paymaster"`
	PaymasterData        []byte         `json:"paymasterData"`
	PaymasterSignature   []byte         `json:"paymasterSignature,omitempty"`
	Signature            []byte         `json:"signature,omitempty"`
	EntryPoint           common.Address `json:"entryPoint"`
	ChainID              *big.Int       `json:"chainId"`
	AuthNonce            uint64         `json:"authNonce"`
	Delegate             common.Address `json:"delegate"`
	RequestID            string         `json:"requestId,omitempty"`
}

// PackedUserOperation matches EntryPoint v0.9 struct.
type PackedUserOperation struct {
	Sender            common.Address
	Nonce             *big.Int
	InitCode          []byte
	CallData          []byte
	AccountGasLimits  [32]byte
	PreVerificationGas *big.Int
	GasFees           [32]byte
	PaymasterAndData  []byte
	Signature         []byte
}

func PackUint128s(high, low *big.Int) [32]byte {
	var out [32]byte
	hi := new(big.Int).Set(high)
	lo := new(big.Int).Set(low)
	hi.Lsh(hi, 128)
	hi.Or(hi, lo)
	hi.FillBytes(out[:])
	return out
}

func PackUserOp(op *UserOperation, forSigning bool) PackedUserOperation {
	paymasterAndData := PackPaymasterAndData(op.Paymaster, op.PaymasterData, op.PaymasterSignature, forSigning)
	initCode := op.InitCode
	if len(initCode) == 0 {
		initCode = common.FromHex(InitCodeEIP7702Marker)
	}
	return PackedUserOperation{
		Sender:             op.Sender,
		Nonce:              new(big.Int).Set(op.Nonce),
		InitCode:           initCode,
		CallData:           op.CallData,
		AccountGasLimits:   PackUint128s(op.VerificationGasLimit, op.CallGasLimit),
		PreVerificationGas: new(big.Int).Set(op.PreVerificationGas),
		GasFees:            PackUint128s(op.MaxPriorityFeePerGas, op.MaxFeePerGas),
		PaymasterAndData:   paymasterAndData,
		Signature:          op.Signature,
	}
}

func PackPaymasterAndData(paymaster common.Address, paymasterData, paymasterSig []byte, forSigning bool) []byte {
	if paymaster == (common.Address{}) {
		return nil
	}
	out := make([]byte, 0, 52+len(paymasterData)+len(paymasterSig)+10)
	out = append(out, paymaster.Bytes()...)
	out = append(out, packUint128Bytes(DefaultPaymasterVerifyGas)...)
	out = append(out, packUint128Bytes(DefaultPaymasterPostOpGas)...)
	out = append(out, paymasterData...)
	if len(paymasterSig) > 0 && !forSigning {
		out = append(out, paymasterSig...)
		sigLen := make([]byte, 2)
		binary.BigEndian.PutUint16(sigLen, uint16(len(paymasterSig)))
		out = append(out, sigLen...)
		out = append(out, common.Hex2Bytes(PaymasterSigMagic)...)
	} else if forSigning && len(paymasterSig) > 0 {
		out = append(out, common.Hex2Bytes(PaymasterSigMagic)...)
	}
	return out
}

func packUint128Bytes(v uint64) []byte {
	b := make([]byte, 16)
	binary.BigEndian.PutUint64(b[8:], v)
	return b
}

func KeccakPaymasterAndData(data []byte) common.Hash {
	sigLen := paymasterSignatureLength(data)
	if sigLen > 0 {
		cut := len(data) - sigLen - 10
		buf := append(common.CopyBytes(data[:cut]), common.Hex2Bytes(PaymasterSigMagic)...)
		return crypto.Keccak256Hash(buf)
	}
	return crypto.Keccak256Hash(data)
}

func paymasterSignatureLength(data []byte) int {
	magic := common.Hex2Bytes(PaymasterSigMagic)
	if len(data) < len(magic)+10 {
		return 0
	}
	suffix := data[len(data)-len(magic):]
	if !bytesEqual(suffix, magic) {
		return 0
	}
	return int(binary.BigEndian.Uint16(data[len(data)-len(magic)-2 : len(data)-len(magic)]))
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func isEIP7702InitCode(initCode []byte) bool {
	marker := common.FromHex(InitCodeEIP7702Marker)
	if len(initCode) < len(marker) {
		return false
	}
	for i := range marker {
		if initCode[i] != marker[i] {
			return false
		}
	}
	return true
}

// initCodeHash matches EntryPoint Eip7702Support._getEip7702InitCodeHashOverride for marker-only initCode.
func initCodeHash(op *UserOperation, initCode []byte) common.Hash {
	if isEIP7702InitCode(initCode) {
		if op.Delegate == (common.Address{}) {
			return crypto.Keccak256Hash(initCode)
		}
		if len(initCode) <= 20 {
			return crypto.Keccak256Hash(op.Delegate.Bytes())
		}
		tail := initCode[20:]
		return crypto.Keccak256Hash(append(op.Delegate.Bytes(), tail...))
	}
	return crypto.Keccak256Hash(initCode)
}

func GetUserOpHash(op *UserOperation) common.Hash {
	packed := PackUserOp(op, true)
	args := abi.Arguments{
		{Type: mustType("bytes32")},
		{Type: mustType("address")}, {Type: mustType("uint256")}, {Type: mustType("bytes32")}, {Type: mustType("bytes32")},
		{Type: mustType("bytes32")}, {Type: mustType("uint256")}, {Type: mustType("bytes32")}, {Type: mustType("bytes32")},
	}
	encoded, _ := args.Pack(
		packedUserOpTypeHash,
		packed.Sender,
		packed.Nonce,
		initCodeHash(op, packed.InitCode),
		crypto.Keccak256Hash(packed.CallData),
		packed.AccountGasLimits,
		packed.PreVerificationGas,
		packed.GasFees,
		KeccakPaymasterAndData(packed.PaymasterAndData),
	)
	domainSeparator := domainSeparator(op.EntryPoint, op.ChainID)
	return crypto.Keccak256Hash(append([]byte{0x19, 0x01}, append(domainSeparator[:], crypto.Keccak256Hash(encoded).Bytes()...)...))
}

func domainSeparator(entryPoint common.Address, chainID *big.Int) common.Hash {
	nameHash := crypto.Keccak256Hash([]byte(DomainName))
	versionHash := crypto.Keccak256Hash([]byte(DomainVersion))
	args := abi.Arguments{
		{Type: mustType("bytes32")}, {Type: mustType("bytes32")}, {Type: mustType("bytes32")}, {Type: mustType("uint256")}, {Type: mustType("address")},
	}
	encoded, _ := args.Pack(eip712DomainTypeHash, nameHash, versionHash, chainID, entryPoint)
	return crypto.Keccak256Hash(encoded)
}

func mustType(t string) abi.Type {
	typ, err := abi.NewType(t, "", nil)
	if err != nil {
		panic(err)
	}
	return typ
}

func EncodeExecuteTokenTransferCall(token, to common.Address, amount *big.Int) ([]byte, error) {
	parsed, err := abi.JSON(strings.NewReader(`[{"name":"executeTokenTransfer","type":"function","inputs":[{"name":"token","type":"address"},{"name":"to","type":"address"},{"name":"amount","type":"uint256"}]}]`))
	if err != nil {
		return nil, err
	}
	return parsed.Pack("executeTokenTransfer", token, to, amount)
}

func UserOpToJSON(op *UserOperation) (string, error) {
	type alias UserOperation
	b, err := json.Marshal((*alias)(op))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func UserOpFromJSON(raw string) (*UserOperation, error) {
	var op UserOperation
	if err := json.Unmarshal([]byte(raw), &op); err != nil {
		return nil, err
	}
	return &op, nil
}
