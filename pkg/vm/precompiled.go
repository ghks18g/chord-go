package vm

import "github.com/ghks18g/chord-go/pkg/types"

// PrecompiledContract is the standard execution interface for native EVM precompiles.
type PrecompiledContract interface {
	RequiredGas(input []byte) uint64
	Run(input []byte) ([]byte, error)
}

// PrecompiledContracts maps native addresses (e.g., 0x100) to their handlers.
var PrecompiledContracts = map[types.Address]PrecompiledContract{
	// RIP-7212 secp256r1 P256VERIFY Precompile at 0x100
	types.HexToAddressMust("0x0000000000000000000000000000000000000100"): &RIP7212Precompile{},
}
