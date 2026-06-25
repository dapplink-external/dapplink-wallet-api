package aa

// NormalizeECDSAV converts go-ethereum crypto.Sign recovery id (0/1) to yellow-paper v (27/28).
// OpenZeppelin ECDSA.recover rejects 65-byte signatures whose v is not 27 or 28.
func NormalizeECDSAV(sig []byte) []byte {
	if len(sig) != 65 || sig[64] >= 27 {
		return sig
	}
	out := append([]byte(nil), sig...)
	out[64] += 27
	return out
}
