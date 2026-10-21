package vm

import (
	"errors"
	"fmt"

	"github.com/ghks18g/chord-go/pkg/types"

	"github.com/holiman/uint256"
	"golang.org/x/crypto/sha3"
)

var (
	ErrOutOfGas            = errors.New("out of gas")
	ErrInvalidOpcode       = errors.New("invalid opcode")
	ErrInvalidJump         = errors.New("invalid jump destination")
	ErrExecutionReverted   = errors.New("execution reverted")
	ErrInvalidCode         = errors.New("invalid bytecode")
	ErrDepth               = errors.New("max call depth exceeded")
	ErrWriteProtection     = errors.New("write protection")
	ErrInsufficientBalance = errors.New("insufficient balance")
)

// Context provides environment data for EVM execution.
type Context struct {
	Caller      types.Address
	To          types.Address
	Origin      types.Address
	BlockNumber int64
	Time        int64
	GasPrice    *uint256.Int
	Value       *uint256.Int
}

// EVM is the core execution machine.
type EVM struct {
	StateDB    StateDB
	Context    Context
	depth      int
	isStatic   bool
	returnData []byte
}

// NewEVM creates a new EVM instance.
func NewEVM(stateDB StateDB, ctx Context) *EVM {
	if ctx.GasPrice == nil {
		ctx.GasPrice = uint256.NewInt(0)
	}
	if ctx.Value == nil {
		ctx.Value = uint256.NewInt(0)
	}
	return &EVM{
		StateDB:  stateDB,
		Context:  ctx,
		depth:    0,
		isStatic: false,
	}
}

// Execute runs the EVM bytecode with the given input and gas limit.
func (evm *EVM) Execute(code []byte, input []byte, gas uint64) (returnData []byte, gasLeft uint64, err error) {
	stack := NewStack()
	memory := NewMemory()
	gasLeft = gas
	pc := uint64(0)
	codeLen := uint64(len(code))

	// Pre-scan JUMPDEST positions
	jumpDests := make(map[uint64]bool)
	for i := uint64(0); i < codeLen; i++ {
		op := OpCode(code[i])
		if op == JUMPDEST {
			jumpDests[i] = true
		} else if op >= PUSH1 && op <= PUSH32 {
			pushSize := uint64(op - PUSH1 + 1)
			i += pushSize
		}
	}

	for pc < codeLen {
		op := OpCode(code[pc])

		// Basic gas deduction
		gasCost := GasBase
		switch {
		case op == STOP:
			gasCost = GasZero
		case op == ADD || op == SUB || op == LT || op == GT || op == EQ || op == ISZERO || op == AND || op == OR || op == XOR || op == NOT:
			gasCost = GasVeryLow
		case op == MUL || op == DIV || op == SDIV || op == MOD:
			gasCost = GasLow
		case op >= PUSH1 && op <= PUSH32:
			gasCost = GasVeryLow
		case op == PUSH0:
			gasCost = GasBase
		case op >= DUP1 && op <= DUP16:
			gasCost = GasVeryLow
		case op >= SWAP1 && op <= SWAP16:
			gasCost = GasVeryLow
		case op == MLOAD || op == MSTORE || op == MSTORE8:
			gasCost = GasVeryLow
		case op == SLOAD:
			gasCost = GasSLoad
		case op == SSTORE:
			gasCost = GasSStore
		case op == JUMP:
			gasCost = GasMid
		case op == JUMPI:
			gasCost = GasHigh
		case op == JUMPDEST:
			gasCost = GasJumpDest
		case op == SHA3:
			gasCost = GasKeccak
		case op == CALL || op == STATICCALL || op == DELEGATECALL:
			gasCost = GasCall
		case op == RETURNDATASIZE || op == RETURNDATACOPY:
			gasCost = GasVeryLow
		case op >= LOG0 && op <= LOG4:
			gasCost = GasZero // dynamic gas computed in opcode handler
		}

		if gasLeft < gasCost {
			return nil, 0, ErrOutOfGas
		}
		gasLeft -= gasCost

		switch op {
		case STOP:
			return nil, gasLeft, nil

		case ADD:
			x, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			y, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			res := new(uint256.Int).Add(x, y)
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case SUB:
			x, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			y, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			res := new(uint256.Int).Sub(x, y)
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case MUL:
			x, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			y, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			res := new(uint256.Int).Mul(x, y)
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case DIV:
			x, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			y, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			res := new(uint256.Int).Div(x, y)
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case MOD:
			x, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			y, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			res := new(uint256.Int).Mod(x, y)
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case LT:
			x, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			y, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			res := uint256.NewInt(0)
			if x.Lt(y) {
				res.SetOne()
			}
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case GT:
			x, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			y, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			res := uint256.NewInt(0)
			if x.Gt(y) {
				res.SetOne()
			}
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case EQ:
			x, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			y, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			res := uint256.NewInt(0)
			if x.Eq(y) {
				res.SetOne()
			}
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case ISZERO:
			x, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			res := uint256.NewInt(0)
			if x.IsZero() {
				res.SetOne()
			}
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case AND:
			x, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			y, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			res := new(uint256.Int).And(x, y)
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case OR:
			x, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			y, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			res := new(uint256.Int).Or(x, y)
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case XOR:
			x, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			y, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			res := new(uint256.Int).Xor(x, y)
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case NOT:
			x, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			res := new(uint256.Int).Not(x)
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case SHA3:
			offset, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			size, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			data := memory.GetCopy(offset.Uint64(), size.Uint64())
			h := sha3.NewLegacyKeccak256()
			h.Write(data)
			var hash [32]byte
			copy(hash[:], h.Sum(nil))
			res := new(uint256.Int).SetBytes32(hash[:])
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case ADDRESS:
			res := new(uint256.Int).SetBytes(evm.Context.To.Bytes())
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case BALANCE:
			addrInt, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			var addr types.Address
			b := addrInt.Bytes20()
			copy(addr[:], b[:])
			bal := uint256.NewInt(0)
			if evm.StateDB != nil {
				bal = evm.StateDB.GetBalance(addr)
			}
			if err := stack.Push(bal); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case ORIGIN:
			res := new(uint256.Int).SetBytes(evm.Context.Origin.Bytes())
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case CALLER:
			res := new(uint256.Int).SetBytes(evm.Context.Caller.Bytes())
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case CALLVALUE:
			if err := stack.Push(evm.Context.Value); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case CALLDATALOAD:
			offset, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			off := offset.Uint64()
			var buf [32]byte
			if off < uint64(len(input)) {
				copy(buf[:], input[off:])
			}
			res := new(uint256.Int).SetBytes32(buf[:])
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case CALLDATASIZE:
			res := uint256.NewInt(uint64(len(input)))
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case CALLDATACOPY:
			memOff, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			dataOff, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			length, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			mOff := memOff.Uint64()
			dOff := dataOff.Uint64()
			l := length.Uint64()
			buf := make([]byte, l)
			if dOff < uint64(len(input)) {
				copy(buf, input[dOff:])
			}
			memory.Set(mOff, l, buf)
			pc++

		case CODESIZE:
			res := uint256.NewInt(uint64(len(code)))
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case CODECOPY:
			memOff, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			codeOff, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			length, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			mOff := memOff.Uint64()
			cOff := codeOff.Uint64()
			l := length.Uint64()
			buf := make([]byte, l)
			if cOff < uint64(len(code)) {
				copy(buf, code[cOff:])
			}
			memory.Set(mOff, l, buf)
			pc++

		case GASPRICE:
			if err := stack.Push(evm.Context.GasPrice); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case POP:
			_, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			pc++

		case MLOAD:
			offset, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			valBytes := memory.GetCopy(offset.Uint64(), 32)
			res := new(uint256.Int).SetBytes(valBytes)
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case MSTORE:
			offset, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			val, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			memory.Set32(offset.Uint64(), val)
			pc++

		case MSTORE8:
			offset, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			val, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			memory.SetByte(offset.Uint64(), byte(val.Uint64()&0xff))
			pc++

		case SLOAD:
			keyInt, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			var key types.Hash
			b := keyInt.Bytes32()
			copy(key[:], b[:])
			valHash := types.EmptyHash()
			if evm.StateDB != nil {
				valHash = evm.StateDB.GetState(evm.Context.To, key)
			}
			res := new(uint256.Int).SetBytes32(valHash.Bytes())
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case SSTORE:
			if evm.isStatic {
				return nil, gasLeft, ErrWriteProtection
			}
			keyInt, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			valInt, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			var key, val types.Hash
			kBytes := keyInt.Bytes32()
			vBytes := valInt.Bytes32()
			copy(key[:], kBytes[:])
			copy(val[:], vBytes[:])
			if evm.StateDB != nil {
				evm.StateDB.SetState(evm.Context.To, key, val)
			}
			pc++

		case JUMP:
			dest, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			d := dest.Uint64()
			if !jumpDests[d] {
				return nil, gasLeft, fmt.Errorf("%w: at pc %d", ErrInvalidJump, d)
			}
			pc = d

		case JUMPI:
			dest, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			cond, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			if !cond.IsZero() {
				d := dest.Uint64()
				if !jumpDests[d] {
					return nil, gasLeft, fmt.Errorf("%w: at pc %d", ErrInvalidJump, d)
				}
				pc = d
			} else {
				pc++
			}

		case PC:
			res := uint256.NewInt(pc)
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case MSIZE:
			res := uint256.NewInt(uint64(memory.Len()))
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case GAS:
			res := uint256.NewInt(gasLeft)
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case JUMPDEST:
			pc++

		case RETURN:
			offset, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			size, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			ret := memory.GetCopy(offset.Uint64(), size.Uint64())
			return ret, gasLeft, nil

		case REVERT:
			offset, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			size, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			ret := memory.GetCopy(offset.Uint64(), size.Uint64())
			return ret, gasLeft, ErrExecutionReverted

		case RETURNDATASIZE:
			res := uint256.NewInt(uint64(len(evm.returnData)))
			if err := stack.Push(res); err != nil {
				return nil, gasLeft, err
			}
			pc++

		case RETURNDATACOPY:
			memOff, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			dataOff, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			length, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			mOff := memOff.Uint64()
			dOff := dataOff.Uint64()
			l := length.Uint64()
			buf := make([]byte, l)
			if dOff < uint64(len(evm.returnData)) {
				copy(buf, evm.returnData[dOff:])
			}
			memory.Set(mOff, l, buf)
			pc++

		case LOG0, LOG1, LOG2, LOG3, LOG4:
			if evm.isStatic {
				return nil, gasLeft, ErrWriteProtection
			}
			nTopics := int(op - LOG0)
			memOff, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			memSize, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}

			offset := memOff.Uint64()
			size := memSize.Uint64()

			logGas := GasLog + GasLogTopic*uint64(nTopics) + GasLogData*size
			if gasLeft < logGas {
				return nil, 0, ErrOutOfGas
			}
			gasLeft -= logGas

			topics := make([]types.Hash, nTopics)
			for i := 0; i < nTopics; i++ {
				top, err := stack.Pop()
				if err != nil {
					return nil, gasLeft, err
				}
				b := top.Bytes32()
				copy(topics[i][:], b[:])
			}

			data := memory.GetCopy(offset, size)

			if evm.StateDB != nil {
				evm.StateDB.AddLog(&types.Log{
					Address:     evm.Context.To,
					Topics:      topics,
					Data:        data,
					BlockNumber: uint64(evm.Context.BlockNumber),
				})
			}
			pc++

		case CALL, STATICCALL, DELEGATECALL:
			gasVal, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			toVal, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}

			var val *uint256.Int
			if op == CALL {
				val, err = stack.Pop()
				if err != nil {
					return nil, gasLeft, err
				}
			} else if op == STATICCALL {
				val = uint256.NewInt(0)
			} else { // DELEGATECALL
				val = evm.Context.Value
			}

			inOffVal, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			inSizeVal, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			outOffVal, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}
			outSizeVal, err := stack.Pop()
			if err != nil {
				return nil, gasLeft, err
			}

			b := toVal.Bytes32()
			toAddr := types.BytesToAddress(b[12:])
			inOff := inOffVal.Uint64()
			inSize := inSizeVal.Uint64()
			outOff := outOffVal.Uint64()
			outSize := outSizeVal.Uint64()

			// Call depth limit
			if evm.depth >= 1024 {
				if err := stack.Push(uint256.NewInt(0)); err != nil {
					return nil, gasLeft, err
				}
				pc++
				continue
			}

			// STATICCALL write protection: cannot transfer value
			if evm.isStatic && op == CALL && !val.IsZero() {
				return nil, gasLeft, ErrWriteProtection
			}

			callGas := gasVal.Uint64()
			if callGas > gasLeft {
				callGas = gasLeft
			}
			gasLeft -= callGas

			var snap int
			if evm.StateDB != nil {
				snap = evm.StateDB.Snapshot()
			}

			// Value transfer for CALL
			if op == CALL && !val.IsZero() {
				if evm.StateDB == nil || evm.StateDB.GetBalance(evm.Context.To).Lt(val) {
					if evm.StateDB != nil {
						evm.StateDB.RevertToSnapshot(snap)
					}
					gasLeft += callGas
					if err := stack.Push(uint256.NewInt(0)); err != nil {
						return nil, gasLeft, err
					}
					pc++
					continue
				}
				evm.StateDB.SubBalance(evm.Context.To, val)
				evm.StateDB.AddBalance(toAddr, val)
			}

			input := memory.GetCopy(inOff, inSize)

			var ret []byte
			var subErr error
			var subGasLeft uint64

			// Route Precompiled Contracts
			if precompile, isPrecompile := PrecompiledContracts[toAddr]; isPrecompile {
				reqGas := precompile.RequiredGas(input)
				if callGas < reqGas {
					if evm.StateDB != nil {
						evm.StateDB.RevertToSnapshot(snap)
					}
					evm.returnData = nil
					if err := stack.Push(uint256.NewInt(0)); err != nil {
						return nil, gasLeft, err
					}
					pc++
					continue
				}
				callGas -= reqGas
				subGasLeft = callGas
				ret, subErr = precompile.Run(input)
			} else {
				var code []byte
				if evm.StateDB != nil {
					code = evm.StateDB.GetCode(toAddr)
				}

				if len(code) == 0 {
					subGasLeft = callGas
					ret = []byte{}
					subErr = nil
				} else {
					var subCtx Context
					var subStatic bool

					switch op {
					case CALL:
						subCtx = Context{
							Caller:      evm.Context.To,
							To:          toAddr,
							Origin:      evm.Context.Origin,
							BlockNumber: evm.Context.BlockNumber,
							Time:        evm.Context.Time,
							GasPrice:    evm.Context.GasPrice,
							Value:       val,
						}
						subStatic = evm.isStatic

					case STATICCALL:
						subCtx = Context{
							Caller:      evm.Context.To,
							To:          toAddr,
							Origin:      evm.Context.Origin,
							BlockNumber: evm.Context.BlockNumber,
							Time:        evm.Context.Time,
							GasPrice:    evm.Context.GasPrice,
							Value:       uint256.NewInt(0),
						}
						subStatic = true

					case DELEGATECALL:
						subCtx = Context{
							Caller:      evm.Context.Caller,
							To:          evm.Context.To,
							Origin:      evm.Context.Origin,
							BlockNumber: evm.Context.BlockNumber,
							Time:        evm.Context.Time,
							GasPrice:    evm.Context.GasPrice,
							Value:       evm.Context.Value,
						}
						subStatic = evm.isStatic
					}

					subEVM := &EVM{
						StateDB:  evm.StateDB,
						Context:  subCtx,
						depth:    evm.depth + 1,
						isStatic: subStatic,
					}
					ret, subGasLeft, subErr = subEVM.Execute(code, input, callGas)
				}
			}

			evm.returnData = ret
			gasLeft += subGasLeft

			if subErr != nil {
				if evm.StateDB != nil {
					evm.StateDB.RevertToSnapshot(snap)
				}
				if errors.Is(subErr, ErrExecutionReverted) && outSize > 0 {
					toCopy := outSize
					if uint64(len(ret)) < toCopy {
						toCopy = uint64(len(ret))
					}
					zeroBuf := make([]byte, outSize)
					copy(zeroBuf, ret[:toCopy])
					memory.Set(outOff, outSize, zeroBuf)
				}
				if err := stack.Push(uint256.NewInt(0)); err != nil {
					return nil, gasLeft, err
				}
			} else {
				if outSize > 0 {
					toCopy := outSize
					if uint64(len(ret)) < toCopy {
						toCopy = uint64(len(ret))
					}
					zeroBuf := make([]byte, outSize)
					copy(zeroBuf, ret[:toCopy])
					memory.Set(outOff, outSize, zeroBuf)
				}
				if err := stack.Push(uint256.NewInt(1)); err != nil {
					return nil, gasLeft, err
				}
			}
			pc++

		case PUSH0:
			if err := stack.Push(uint256.NewInt(0)); err != nil {
				return nil, gasLeft, err
			}
			pc++

		default:
			// PUSH1 to PUSH32
			if op >= PUSH1 && op <= PUSH32 {
				n := uint64(op - PUSH1 + 1)
				if pc+1+n > codeLen {
					return nil, gasLeft, ErrInvalidCode
				}
				data := code[pc+1 : pc+1+n]
				val := new(uint256.Int).SetBytes(data)
				if err := stack.Push(val); err != nil {
					return nil, gasLeft, err
				}
				pc += 1 + n
				continue
			}

			// DUP1 to DUP16
			if op >= DUP1 && op <= DUP16 {
				n := int(op - DUP1 + 1)
				if err := stack.Dup(n); err != nil {
					return nil, gasLeft, err
				}
				pc++
				continue
			}

			// SWAP1 to SWAP16
			if op >= SWAP1 && op <= SWAP16 {
				n := int(op - SWAP1 + 1)
				if err := stack.Swap(n); err != nil {
					return nil, gasLeft, err
				}
				pc++
				continue
			}

			return nil, gasLeft, fmt.Errorf("%w: opcode 0x%02x at pc %d", ErrInvalidOpcode, byte(op), pc)
		}
	}

	return nil, gasLeft, nil
}
