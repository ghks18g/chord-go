package vm

import (
	gethcommon "github.com/ethereum/go-ethereum/common"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ghks18g/chord-go/pkg/crypto/poseidon"
	"github.com/ghks18g/chord-go/pkg/types"
)

// PrecompiledContract is the standard execution interface for native EVM precompiles.
type PrecompiledContract interface {
	RequiredGas(input []byte) uint64
	Run(input []byte) ([]byte, error)
}

// PoseidonPrecompile implements 0x200 Poseidon-BN254 Hash2.
type PoseidonPrecompile struct{}

func (p *PoseidonPrecompile) RequiredGas(input []byte) uint64 {
	return 500
}

func (p *PoseidonPrecompile) Run(input []byte) ([]byte, error) {
	var left, right [32]byte
	if len(input) >= 32 {
		copy(left[:], input[:32])
	}
	if len(input) >= 64 {
		copy(right[:], input[32:64])
	}
	res := poseidon.Hash2(left, right)
	return res[:], nil
}

// PrecompiledContracts maps native addresses (e.g., 0x1..0x9, 0x100, 0x200) to their handlers.
var PrecompiledContracts = map[types.Address]PrecompiledContract{}

func init() {
	// Standard Ethereum Precompiles 0x1 ~ 0x9 from Cancun set
	for i := 1; i <= 9; i++ {
		addr := types.BytesToAddress([]byte{byte(i)})
		gethAddr := gethcommon.BytesToAddress([]byte{byte(i)})
		if c, ok := gethvm.PrecompiledContractsCancun[gethAddr]; ok {
			PrecompiledContracts[addr] = c
		}
	}

	// 0x100: RIP-7212 secp256r1 P256VERIFY Precompile
	PrecompiledContracts[types.HexToAddressMust("0x0000000000000000000000000000000000000100")] = &RIP7212Precompile{}

	// 0x200: Poseidon-BN254 Hash2 Precompile for CHORD SMT
	PrecompiledContracts[types.HexToAddressMust("0x0000000000000000000000000000000000000200")] = &PoseidonPrecompile{}
}
