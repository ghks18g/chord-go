package vm

import "fmt"

// OpCode represents an EVM instruction byte.
type OpCode byte

const (
	STOP OpCode = 0x00
	ADD  OpCode = 0x01
	MUL  OpCode = 0x02
	SUB  OpCode = 0x03
	DIV  OpCode = 0x04
	SDIV OpCode = 0x05
	MOD  OpCode = 0x06
	SMOD OpCode = 0x07

	LT     OpCode = 0x10
	GT     OpCode = 0x11
	SLT    OpCode = 0x12
	SGT    OpCode = 0x13
	EQ     OpCode = 0x14
	ISZERO OpCode = 0x15
	AND    OpCode = 0x16
	OR     OpCode = 0x17
	XOR    OpCode = 0x18
	NOT    OpCode = 0x19
	BYTE   OpCode = 0x1a
	SHL    OpCode = 0x1b
	SHR    OpCode = 0x1c

	SHA3 OpCode = 0x20

	ADDRESS      OpCode = 0x30
	BALANCE      OpCode = 0x31
	ORIGIN       OpCode = 0x32
	CALLER       OpCode = 0x33
	CALLVALUE    OpCode = 0x34
	CALLDATALOAD OpCode = 0x35
	CALLDATASIZE OpCode = 0x36
	CALLDATACOPY OpCode = 0x37
	CODESIZE     OpCode = 0x38
	CODECOPY     OpCode = 0x39
	GASPRICE     OpCode = 0x3a

	POP      OpCode = 0x50
	MLOAD    OpCode = 0x51
	MSTORE   OpCode = 0x52
	MSTORE8  OpCode = 0x53
	SLOAD    OpCode = 0x54
	SSTORE   OpCode = 0x55
	JUMP     OpCode = 0x56
	JUMPI    OpCode = 0x57
	PC       OpCode = 0x58
	MSIZE    OpCode = 0x59
	GAS      OpCode = 0x5a
	JUMPDEST OpCode = 0x5b

	PUSH1  OpCode = 0x60
	PUSH2  OpCode = 0x61
	PUSH3  OpCode = 0x62
	PUSH4  OpCode = 0x63
	PUSH32 OpCode = 0x7f

	DUP1  OpCode = 0x80
	DUP2  OpCode = 0x81
	DUP16 OpCode = 0x8f

	SWAP1  OpCode = 0x90
	SWAP2  OpCode = 0x91
	SWAP16 OpCode = 0x9f

	RETURN OpCode = 0xf3
	REVERT OpCode = 0xfd
)

// Gas costs
const (
	GasZero     uint64 = 0
	GasBase     uint64 = 2
	GasVeryLow  uint64 = 3
	GasLow      uint64 = 5
	GasMid      uint64 = 8
	GasHigh     uint64 = 10
	GasJumpDest uint64 = 1
	GasSStore   uint64 = 100
	GasSLoad    uint64 = 50
	GasKeccak   uint64 = 30
)

// String returns the human readable opcode name.
func (op OpCode) String() string {
	if op >= PUSH1 && op <= PUSH32 {
		return fmt.Sprintf("PUSH%d", op-PUSH1+1)
	}
	if op >= DUP1 && op <= DUP16 {
		return fmt.Sprintf("DUP%d", op-DUP1+1)
	}
	if op >= SWAP1 && op <= SWAP16 {
		return fmt.Sprintf("SWAP%d", op-SWAP1+1)
	}

	switch op {
	case STOP:
		return "STOP"
	case ADD:
		return "ADD"
	case MUL:
		return "MUL"
	case SUB:
		return "SUB"
	case DIV:
		return "DIV"
	case SDIV:
		return "SDIV"
	case MOD:
		return "MOD"
	case LT:
		return "LT"
	case GT:
		return "GT"
	case EQ:
		return "EQ"
	case ISZERO:
		return "ISZERO"
	case AND:
		return "AND"
	case OR:
		return "OR"
	case XOR:
		return "XOR"
	case NOT:
		return "NOT"
	case SHA3:
		return "SHA3"
	case ADDRESS:
		return "ADDRESS"
	case BALANCE:
		return "BALANCE"
	case ORIGIN:
		return "ORIGIN"
	case CALLER:
		return "CALLER"
	case CALLVALUE:
		return "CALLVALUE"
	case CALLDATALOAD:
		return "CALLDATALOAD"
	case CALLDATASIZE:
		return "CALLDATASIZE"
	case CALLDATACOPY:
		return "CALLDATACOPY"
	case POP:
		return "POP"
	case MLOAD:
		return "MLOAD"
	case MSTORE:
		return "MSTORE"
	case MSTORE8:
		return "MSTORE8"
	case SLOAD:
		return "SLOAD"
	case SSTORE:
		return "SSTORE"
	case JUMP:
		return "JUMP"
	case JUMPI:
		return "JUMPI"
	case PC:
		return "PC"
	case MSIZE:
		return "MSIZE"
	case GAS:
		return "GAS"
	case JUMPDEST:
		return "JUMPDEST"
	case RETURN:
		return "RETURN"
	case REVERT:
		return "REVERT"
	default:
		return fmt.Sprintf("UNKNOWN(0x%x)", byte(op))
	}
}
