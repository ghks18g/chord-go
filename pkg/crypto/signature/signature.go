package signature

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"

	"github.com/ghks18g/chord-go/pkg/types"
	"golang.org/x/crypto/sha3"
)

// Keccak256 calculates the Keccak-256 hash of the input data.
func Keccak256(data ...[]byte) [32]byte {
	d := sha3.NewLegacyKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	var out [32]byte
	copy(out[:], d.Sum(nil))
	return out
}

// EIP712Domain defines the typed data domain separator fields.
type EIP712Domain struct {
	Name              string
	Version           string
	ChainID           uint64
	VerifyingContract types.Address
}

// Separator computes the EIP-712 domain separator hash.
func (d *EIP712Domain) Separator() [32]byte {
	typeHash := Keccak256([]byte("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"))
	nameHash := Keccak256([]byte(d.Name))
	versionHash := Keccak256([]byte(d.Version))

	var chainIDBytes [32]byte
	// encode chainId as big-endian 32-byte
	for i := 0; i < 8; i++ {
		chainIDBytes[31-i] = byte(d.ChainID >> (i * 8))
	}

	var addrBytes [32]byte
	copy(addrBytes[12:], d.VerifyingContract[:])

	return Keccak256(
		typeHash[:],
		nameHash[:],
		versionHash[:],
		chainIDBytes[:],
		addrBytes[:],
	)
}

// HashTypedData hashes structured data according to EIP-712:
// \x19\x01 || domainSeparator || hashStruct(message)
func HashTypedData(domain *EIP712Domain, structHash [32]byte) [32]byte {
	domainSep := domain.Separator()
	prefix := []byte{0x19, 0x01}
	return Keccak256(prefix, domainSep[:], structHash[:])
}

// PubKeyToAddress derives a 20-byte Ethereum-compatible address from ECDSA public key.
func PubKeyToAddress(pub *ecdsa.PublicKey) types.Address {
	pubBytes := elliptic.Marshal(pub.Curve, pub.X, pub.Y)[1:] // uncompressed without prefix 0x04
	h := Keccak256(pubBytes)
	var addr types.Address
	copy(addr[:], h[12:])
	return addr
}

// GenerateTestKeyPair generates a secp256k1 keypair for testing.
func GenerateTestKeyPair() (*ecdsa.PrivateKey, types.Address, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader) // Standard curve for mock testing
	if err != nil {
		return nil, types.Address{}, err
	}
	return key, PubKeyToAddress(&key.PublicKey), nil
}

// EIP1271MagicValue is bytes4(keccak256("isValidSignature(bytes32,bytes)")) -> 0x1626ba7e
var EIP1271MagicValue = [4]byte{0x16, 0x26, 0xba, 0x7e}

// MockSmartAccountVerifier simulates an on-chain smart account verifying EIP-1271 ZK or contract signatures.
type MockSmartAccountVerifier struct {
	AllowedContract types.Address
	ExpectedProof   []byte
}

func (v *MockSmartAccountVerifier) IsValidSignature(account types.Address, hash [32]byte, sigOrProof []byte) (bool, error) {
	if account != v.AllowedContract {
		return false, errors.New("contract address mismatch")
	}
	if !bytes.Equal(sigOrProof, v.ExpectedProof) {
		return false, errors.New("invalid signature or zk proof payload")
	}
	return true, nil
}
