package signature

import (
	"crypto/ecdsa"
	"crypto/rand"
	"testing"

	"github.com/ghks18g/chord-go/pkg/types"
)

func TestEIP712(t *testing.T) {
	contractAddr, _ := types.HexToAddress("0x1234567890123456789012345678901234567890")
	domain := &EIP712Domain{
		Name:              "CHORD Protocol",
		Version:           "1",
		ChainID:           7613,
		VerifyingContract: contractAddr,
	}

	sep1 := domain.Separator()
	sep2 := domain.Separator()
	if sep1 != sep2 {
		t.Fatalf("domain separator must be deterministic")
	}

	structHash := Keccak256([]byte("Transfer(address to,uint256 amount)"))
	typedHash := HashTypedData(domain, structHash)
	if typedHash == [32]byte{} {
		t.Fatalf("expected non-zero typed data hash")
	}

	priv, addr, err := GenerateTestKeyPair()
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	r, s, err := ecdsa.Sign(rand.Reader, priv, typedHash[:])
	if err != nil {
		t.Fatalf("signing failed: %v", err)
	}

	if !ecdsa.Verify(&priv.PublicKey, typedHash[:], r, s) {
		t.Fatalf("ecdsa signature verification failed")
	}

	if addr.String() == "" {
		t.Fatalf("address string should not be empty")
	}
}

func TestEIP1271(t *testing.T) {
	contractAddr, _ := types.HexToAddress("0x9999999999999999999999999999999999999999")
	zkProofPayload := []byte("groth16-zk-proof-mock-payload-bytes")

	verifier := &MockSmartAccountVerifier{
		AllowedContract: contractAddr,
		ExpectedProof:   zkProofPayload,
	}

	msgHash := Keccak256([]byte("verify-account-abstraction"))

	// Valid proof test
	ok, err := verifier.IsValidSignature(contractAddr, msgHash, zkProofPayload)
	if err != nil || !ok {
		t.Fatalf("EIP-1271 valid signature check failed: %v", err)
	}

	// Invalid proof test
	fakeProof := []byte("fake-proof-payload")
	ok, err = verifier.IsValidSignature(contractAddr, msgHash, fakeProof)
	if err == nil || ok {
		t.Fatalf("EIP-1271 fake proof must be rejected")
	}
}
