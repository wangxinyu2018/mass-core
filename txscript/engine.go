// Copyright (c) 2013-2015 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	"github.com/wangxinyu2018/mass-core/consensus"
	"github.com/wangxinyu2018/mass-core/logging"
	"github.com/wangxinyu2018/mass-core/wire"
)

// halforder is used to tame ECDSA malleability (see BIP0062).
var halfOrder = new(big.Int).Rsh(btcec.S256().N, 1)

// Engine is the virtual machine that executes scripts.
type Engine struct {
	scripts     [][]parsedOpcode
	scriptIdx   int
	scriptOff   int
	lastCodeSep int
	dstack      stack // data stack
	astack      stack // alt stack
	tx          wire.MsgTx
	txIdx       int
	condStack   []int
	// savedFirstStack [][]byte
	numOps         int
	flags          ScriptFlags
	sigCache       *SigCache
	hashCache      *TxSigHashes
	witnessVersion int
	witnessProgram []byte
	witnessExtProg []parsedOpcode // witness extension program
	inputAmount    int64
}

//addScript only to test
func (vm *Engine) addScript(script []parsedOpcode) {
	vm.scripts = append(vm.scripts, script)
}

// hasFlag returns whether the script engine instance has the passed flag set.
func (vm *Engine) hasFlag(flag ScriptFlags) bool {
	return vm.flags&flag == flag
}

// isBranchExecuting returns whether or not the current conditional branch is
// actively executing.  For example, when the data stack has an OP_FALSE on it
// and an OP_IF is encountered, the branch is inactive until an OP_ELSE or
// OP_ENDIF is encountered.  It properly handles nested conditionals.
func (vm *Engine) isBranchExecuting() bool {
	if len(vm.condStack) == 0 {
		return true
	}
	return vm.condStack[len(vm.condStack)-1] == OpCondTrue
}

// executeOpcode peforms execution on the passed opcode.  It takes into account
// whether or not it is hidden by conditionals, but some rules still must be
// tested in this case.
func (vm *Engine) executeOpcode(pop *parsedOpcode) error {
	// Disabled opcodes are fail on program counter.
	if pop.isDisabled() {
		return ErrStackOpDisabled
	}

	// Always-illegal opcodes are fail on program counter.
	if pop.alwaysIllegal() {
		return ErrStackReservedOpcode
	}

	// Note that this includes OP_RESERVED which counts as a push operation.
	if pop.opcode.value > OP_16 {
		vm.numOps++
		if vm.numOps > MaxOpsPerScript {
			return ErrStackTooManyOperations
		}

	} else if len(pop.data) > MaxScriptElementSize {
		return ErrStackElementTooBig
	}

	// Nothing left to do when this is not a conditional opcode and it is
	// not in an executing branch.
	if !vm.isBranchExecuting() && !pop.isConditional() {
		return nil
	}

	// Ensure all executed data push opcodes use the minimal encoding when
	// the minimal data verification flag is set.
	if vm.dstack.verifyMinimalData && vm.isBranchExecuting() &&
		pop.opcode.value >= 0 && pop.opcode.value <= OP_PUSHDATA4 {

		if err := pop.checkMinimalDataPush(); err != nil {
			return err
		}
	}
	//log.Info("the opcode.data is :",pop.data,"the opcode.value is :",pop.opcode.value,"the opcode.name is :",pop.opcode.name)
	return pop.opcode.opfunc(pop, vm)
}

// disasm is a helper function to produce the output for DisasmPC and
// DisasmScript.  It produces the opcode prefixed by the program counter at the
// provided position in the script.  It does no error checking and leaves that
// to the caller to provide a valid offset.
func (vm *Engine) disasm(scriptIdx int, scriptOff int) string {
	return fmt.Sprintf("%02x:%04x: %s", scriptIdx, scriptOff,
		vm.scripts[scriptIdx][scriptOff].print(false))
}

// validPC returns an error if the current script position is valid for
// execution, nil otherwise.
func (vm *Engine) validPC() error {
	if vm.scriptIdx >= len(vm.scripts) {
		return fmt.Errorf("past input scripts %v:%v %v:xxxx",
			vm.scriptIdx, vm.scriptOff, len(vm.scripts))
	}
	if vm.scriptOff >= len(vm.scripts[vm.scriptIdx]) {
		return fmt.Errorf("past input scripts %v:%v %v:%04d",
			vm.scriptIdx, vm.scriptOff, vm.scriptIdx,
			len(vm.scripts[vm.scriptIdx]))
	}
	return nil
}

// isWitnessVersionActive returns true if a witness program was extracted
// during the initialization of the Engine, and the program's version matches
// the specified version.
func (vm *Engine) isWitnessVersionActive(version uint) bool {
	return vm.witnessProgram != nil && uint(vm.witnessVersion) == version
}

// curPC returns either the current script and offset, or an error if the
// position isn't valid.
func (vm *Engine) curPC() (script int, off int, err error) {
	err = vm.validPC()
	if err != nil {
		return 0, 0, err
	}
	return vm.scriptIdx, vm.scriptOff, nil
}

// DisasmPC returns the string for the disassembly of the opcode that will be
// next to execute when Step() is called.
func (vm *Engine) DisasmPC() (string, error) {
	scriptIdx, scriptOff, err := vm.curPC()
	if err != nil {
		return "", err
	}
	return vm.disasm(scriptIdx, scriptOff), nil
}

// DisasmScript returns the disassembly string for the script at the requested
// offset index.  Index 0 is the signature script and 1 is the public key
// script.
func (vm *Engine) DisasmScript(idx int) (string, error) {
	if idx >= len(vm.scripts) {
		return "", ErrStackInvalidIndex
	}

	var disstr string
	for i := range vm.scripts[idx] {
		disstr = disstr + vm.disasm(idx, i) + "\n"
	}
	return disstr, nil
}

// CheckErrorCondition returns nil if the running script has ended and was
// successful, leaving a a true boolean on the stack.  An error otherwise,
// including if the script has not finished.
func (vm *Engine) CheckErrorCondition(finalScript bool) error {
	// Check execution is actually done.  When pc is past the end of script
	// array there are no more scripts to run.
	if vm.scriptIdx < len(vm.scripts) {
		return ErrStackScriptUnfinished
	}
	if finalScript && vm.dstack.Depth() != 1 {

		return ErrStackCleanStack
	} else if vm.dstack.Depth() < 1 {
		return ErrStackEmptyStack
	}

	v, err := vm.dstack.PopBool()

	if err != nil {
		return err
	}
	if !v {
		/// Log interesting data.
		dis0, _ := vm.DisasmScript(0)
		dis1, _ := vm.DisasmScript(1)
		logging.CPrint(logging.ERROR, "scripts failed", logging.LogFormat{"script0 : ": dis0, "script1": dis1})
		return ErrStackScriptFailed
	}
	return nil
}

// Step will execute the next instruction and move the program counter to the
// next opcode in the script, or the next script if the current has ended.  Step
// will return true in the case that the last opcode was successfully executed.
//
// The result of calling Step or any other method is undefined if an error is
// returned.
func (vm *Engine) Step() (done bool, err error) {
	// Verify that it is pointing to a valid script address.
	err = vm.validPC()
	if err != nil {
		return true, err
	}
	opcode := &vm.scripts[vm.scriptIdx][vm.scriptOff]
	vm.scriptOff++

	// Execute the opcode while taking into account several things such as
	// disabled opcodes, illegal opcodes, maximum allowed operations per
	// script, maximum script element sizes, and conditionals.
	err = vm.executeOpcode(opcode)
	if err != nil {
		return true, err
	}

	// The number of elements in the combination of the data and alt stacks
	// must not exceed the maximum number of stack elements allowed.
	if vm.dstack.Depth()+vm.astack.Depth() > maxStackSize {
		return false, ErrStackOverflow
	}

	// Prepare for next instruction.
	if vm.scriptOff >= len(vm.scripts[vm.scriptIdx]) {
		// Illegal to have an `if' that straddles two scripts.
		if err == nil && len(vm.condStack) != 0 {
			return false, ErrStackMissingEndif
		}

		// Alt stack doesn't persist.
		_ = vm.astack.DropN(vm.astack.Depth())

		vm.numOps = 0 // number of ops is per script.
		vm.scriptOff = 0
		if vm.scriptIdx == 0 && vm.witnessProgram != nil {
			vm.scriptIdx++

			if vm.dstack.Depth() != 2 && vm.dstack.Depth() != 3 {
				return false, ErrStackUnderflow
			}
			_ = vm.dstack.DropN(vm.dstack.Depth())
			witness := vm.tx.TxIn[vm.txIdx].Witness
			if err := vm.verifyWitnessProgram(witness); err != nil {
				return false, err
			}
		} else {
			vm.scriptIdx++
		}

		// vm.scriptIdx++
		// if vm.scriptIdx == 2 && vm.witnessVersion >= 10 {
		// 	// Check script ran successfully and pull the script
		// 	// out of the first stack and execute that.
		// 	err := vm.CheckErrorCondition(false)
		// 	if err != nil {
		// 		return false, err
		// 	}
		// 	//push the redeemScript into stack
		// 	pops, err := parseScript(vm.witnessProgram)
		// 	if err != nil {
		// 		return false, err
		// 	}
		// 	vm.scripts = append(vm.scripts, pops)
		// 	vm.SetStack(vm.savedFirstStack[1:])
		// }

		// there are zero length scripts in the wild
		if vm.scriptIdx < len(vm.scripts) && vm.scriptOff >= len(vm.scripts[vm.scriptIdx]) {
			vm.scriptIdx++
		}
		vm.lastCodeSep = 0
		if vm.scriptIdx >= len(vm.scripts) {
			return true, nil
		}
	}
	return false, nil
}

// Execute will execute all scripts in the script engine and return either nil
// for successful validation or an error if one occurred.
func (vm *Engine) Execute() (err error) {
	done := false
	for !done {
		dis, err := vm.DisasmPC()
		logging.CPrint(logging.TRACE, "stepping", logging.LogFormat{
			"script0": dis,
			"error":   err})
		done, err = vm.Step()
		if err != nil {
			return err
		}
		logging.CPrint(logging.TRACE, "stepping", logging.LogFormat{
			"dStack depth":   vm.dstack.Depth(),
			"dStack":         vm.dstack.String(),
			"AltStack depth": vm.astack.Depth(),
			"AltStack":       vm.astack.String(),
		})

	}
	return vm.CheckErrorCondition(true)
}

// subScript returns the script since the last OP_CODESEPARATOR.
func (vm *Engine) subScript() []parsedOpcode {
	return vm.scripts[vm.scriptIdx][vm.lastCodeSep:]
}

// checkHashTypeEncoding returns whether or not the passed hashtype adheres to
// the strict encoding requirements if enabled.
func (vm *Engine) checkHashTypeEncoding(hashType SigHashType) error {
	//if !vm.hasFlag(ScriptVerifyStrictEncoding) {
	//	return nil
	//}
	sigHashType := hashType & ^SigHashAnyOneCanPay
	if sigHashType < SigHashAll || sigHashType > SigHashSingle {
		return fmt.Errorf("invalid hashtype: 0x%x", hashType)
	}
	return nil
}

// checkPubKeyEncoding returns whether or not the passed public key adheres to
// the strict encoding requirements if enabled.
func (vm *Engine) checkPubKeyEncoding(pubKey []byte) error {
	if vm.isWitnessVersionActive(0) && !btcec.IsCompressedPubKey(pubKey) {
		return ErrWitnessPubKeyType
	}
	//if !vm.hasFlag(ScriptVerifyStrictEncoding) {
	//	return nil
	//}

	if len(pubKey) == 33 && (pubKey[0] == 0x02 || pubKey[0] == 0x03) {
		// Compressed
		return nil
	}
	if len(pubKey) == 65 && pubKey[0] == 0x04 {
		// Uncompressed
		return nil
	}
	return ErrStackInvalidPubKey
}

// checkSignatureEncoding returns whether or not the passed signature adheres to
// the strict encoding requirements if enabled.
func (vm *Engine) checkSignatureEncoding(sig []byte) error {

	// The format of a DER encoded signature is as follows:
	//
	// 0x30 <total length> 0x02 <length of R> <R> 0x02 <length of S> <S>
	//   - 0x30 is the ASN.1 identifier for a sequence
	//   - Total length is 1 byte and specifies length of all remaining data
	//   - 0x02 is the ASN.1 identifier that specifies an integer follows
	//   - Length of R is 1 byte and specifies how many bytes R occupies
	//   - R is the arbitrary length big-endian encoded number which
	//     represents the R value of the signature.  DER encoding dictates
	//     that the value must be encoded using the minimum possible number
	//     of bytes.  This implies the first byte can only be null if the
	//     highest bit of the next byte is set in order to prevent it from
	//     being interpreted as a negative number.
	//   - 0x02 is once again the ASN.1 integer identifier
	//   - Length of S is 1 byte and specifies how many bytes S occupies
	//   - S is the arbitrary length big-endian encoded number which
	//     represents the S value of the signature.  The encoding rules are
	//     identical as those for R.

	// Minimum length is when both numbers are 1 byte each.
	// 0x30 + <1-byte> + 0x02 + 0x01 + <byte> + 0x2 + 0x01 + <byte>
	if len(sig) < 8 {
		// Too short
		return fmt.Errorf("malformed signature: too short: %d < 8",
			len(sig))
	}

	// Maximum length is when both numbers are 33 bytes each.  It is 33
	// bytes because a 256-bit integer requires 32 bytes and an additional
	// leading null byte might required if the high bit is set in the value.
	// 0x30 + <1-byte> + 0x02 + 0x21 + <33 bytes> + 0x2 + 0x21 + <33 bytes>
	if len(sig) > 72 {
		// Too long
		return fmt.Errorf("malformed signature: too long: %d > 72",
			len(sig))
	}
	if sig[0] != 0x30 {
		// Wrong type
		return fmt.Errorf("malformed signature: format has wrong type: 0x%x",
			sig[0])
	}
	if int(sig[1]) != len(sig)-2 {
		// Invalid length
		return fmt.Errorf("malformed signature: bad length: %d != %d",
			sig[1], len(sig)-2)
	}

	rLen := int(sig[3])

	// Make sure S is inside the signature.
	if rLen+5 > len(sig) {
		return fmt.Errorf("malformed signature: S out of bounds")
	}

	sLen := int(sig[rLen+5])

	// The length of the elements does not match the length of the
	// signature.
	if rLen+sLen+6 != len(sig) {
		return fmt.Errorf("malformed signature: invalid R length")
	}

	// R elements must be integers.
	if sig[2] != 0x02 {
		return fmt.Errorf("malformed signature: missing first integer marker")
	}

	// Zero-length integers are not allowed for R.
	if rLen == 0 {
		return fmt.Errorf("malformed signature: R length is zero")
	}

	// R must not be negative.
	if sig[4]&0x80 != 0 {
		return fmt.Errorf("malformed signature: R value is negative")
	}

	// Null bytes at the start of R are not allowed, unless R would
	// otherwise be interpreted as a negative number.
	if rLen > 1 && sig[4] == 0x00 && sig[5]&0x80 == 0 {
		return fmt.Errorf("malformed signature: invalid R value")
	}

	// S elements must be integers.
	if sig[rLen+4] != 0x02 {
		return fmt.Errorf("malformed signature: missing second integer marker")
	}

	// Zero-length integers are not allowed for S.
	if sLen == 0 {
		return fmt.Errorf("malformed signature: S length is zero")
	}

	// S must not be negative.
	if sig[rLen+6]&0x80 != 0 {
		return fmt.Errorf("malformed signature: S value is negative")
	}

	// Null bytes at the start of S are not allowed, unless S would
	// otherwise be interpreted as a negative number.
	if sLen > 1 && sig[rLen+6] == 0x00 && sig[rLen+7]&0x80 == 0 {
		return fmt.Errorf("malformed signature: invalid S value")
	}

	// Verify the S value is <= half the order of the curve.  This check is
	// done because when it is higher, the complement modulo the order can
	// be used instead which is a shorter encoding by 1 byte.  Further,
	// without enforcing this, it is possible to replace a signature in a
	// valid transaction with the complement while still being a valid
	// signature that verifies.  This would result in changing the
	// transaction hash and thus is source of malleability.

	sValue := new(big.Int).SetBytes(sig[rLen+6 : rLen+6+sLen])
	if sValue.Cmp(halfOrder) > 0 {
		return ErrStackInvalidLowSSignature
	}

	return nil
}

// getStack returns the contents of stack as a byte array bottom up
func getStack(stack *stack) [][]byte {
	array := make([][]byte, stack.Depth())
	for i := range array {
		// PeekByteArry can't fail due to overflow, already checked
		array[len(array)-i-1], _ = stack.PeekByteArray(int32(i))
	}
	return array
}

// setStack sets the stack to the contents of the array where the last item in
// the array is the top item in the stack.
func setStack(stack *stack, data [][]byte) {
	// This can not error. Only errors are for invalid arguments.
	_ = stack.DropN(stack.Depth())
	for i := range data {
		stack.PushByteArray(data[i])
	}
}

// GetStack returns the contents of the primary stack as an array. where the
// last item in the array is the top of the stack.
func (vm *Engine) GetStack() [][]byte {
	return getStack(&vm.dstack)
}

// SetStack sets the contents of the primary stack to the contents of the
// provided array where the last item in the array will be the top of the stack.
func (vm *Engine) SetStack(data [][]byte) {
	setStack(&vm.dstack, data)
}

// verifyWitnessProgram validates the stored witness program using the passed
// witness as input.
func (vm *Engine) verifyWitnessProgram(witness [][]byte) error {
	if !vm.isWitnessVersionActive(0) {
		return ErrDiscourageUpgradableWitnessProgram
	}

	// Additionally, The witness stack MUST NOT be empty at
	// this point.
	if len(witness) != 2 {
		return ErrWitnessLength
	}

	// When the witness is empty the result is necessarily an error
	// since the stack would end up being
	// empty which is equivalent to a false top element.  Thus, just return
	// the relevant error now as an optimization.
	if len(witness[0]) == 0 || len(witness[1]) == 0 {
		return ErrWitnessLength
	}

	// Ensure that the serialized pkScript at the end of
	// the witness stack matches the witness program.
	witnessHash := sha256.Sum256(witness[1])
	if !bytes.Equal(witnessHash[:], vm.witnessProgram) {
		return ErrWitnessProgramMismatch
	}

	switch len(vm.witnessExtProg) {
	case 0:
		// standard P2WSH, do nothing
	case 1:
		// staking tx
		if len(vm.witnessExtProg[0].data) == WitnessV0FrozenPeriodDataSize {
			// the evaluation locktime for OP_CHECKSEQUENCEVERIFY MUST BE frozenPeriod+1
			frozenPeriod, err := makeScriptNum(vm.witnessExtProg[0].data, vm.dstack.verifyMinimalData, 9)
			if err != nil {
				return err
			}
			if frozenPeriod < 0 || frozenPeriod+1 < 0 {
				return fmt.Errorf("invalid frozen period: %d", frozenPeriod)
			}
			frozenPeriod++
			buf := make([]byte, 8)
			binary.LittleEndian.PutUint64(buf, uint64(frozenPeriod))

			pkScript, err := NewScriptBuilder().AddData(buf).AddOp(OP_CHECKSEQUENCEVERIFY).AddOp(OP_DROP).Script()
			if err != nil {
				return err
			}
			pops, err := parseScript(pkScript)
			if err != nil {
				return err
			}
			vm.scripts = append(vm.scripts, pops)
			break
		}

		// binding tx
		if len(vm.witnessExtProg[0].data) == OP_DATA_20 || len(vm.witnessExtProg[0].data) == OP_DATA_22 {
			if vm.hasFlag(ScriptMASSip2) {
				buf := make([]byte, 8)
				binary.LittleEndian.PutUint64(buf, consensus.MASSIP0002BindingLockedPeriod)
				pkScript, err := NewScriptBuilder().AddData(buf).AddOp(OP_CHECKSEQUENCEVERIFY).AddOp(OP_DROP).Script()
				if err != nil {
					return err
				}
				pops, err := parseScript(pkScript)
				if err != nil {
					return err
				}
				vm.scripts = append(vm.scripts, pops)
			}
			break
		}
		// else error occurs
		fallthrough
	default:
		return ErrWitnessExtProgUnknown
	}

	for _, script := range witness {
		if len(script) > maxScriptSize {
			return ErrScriptTooBig
		}
		// With all the validity checks passed, parse the
		// script into individual op-codes so w can execute it
		// as the next script.
		pops, err := parseScript(script)
		if err != nil {
			return err
		}

		// The hash matched successfully, so use the witness as
		// the stack, and set the redeemScript to be the next
		// script executed.
		vm.scripts = append(vm.scripts, pops)
	}

	// All elements within the witness stack must not be greater
	// than the maximum bytes which are allowed to be pushed onto
	// the stack.
	for _, witElement := range vm.GetStack() {
		if len(witElement) > MaxScriptElementSize {
			return ErrStackElementTooBig
		}
	}

	return nil
}

// NewEngine returns a new script engine for the provided public key script,
// transaction, and input index.  The flags modify the behavior of the script
// engine according to the description provided by each flag.
func NewEngine(pkScript []byte, tx *wire.MsgTx, txIdx int, flags ScriptFlags,
	sigCache *SigCache, hashCache *TxSigHashes, inputAmount int64) (*Engine, error) {
	// The provided transaction input index must refer to a valid input.
	if txIdx < 0 || txIdx >= len(tx.TxIn) {
		return nil, ErrStackInvalidIndex
	}

	if len(pkScript) == 0 {
		return nil, ErrWitnessUnexpected
	}

	vm := Engine{flags: flags, sigCache: sigCache, hashCache: hashCache,
		inputAmount: inputAmount}

	// vm.dstack.verifyMinimalData = true
	// vm.astack.verifyMinimalData = true

	scripts := [][]byte{pkScript}
	vm.scripts = make([][]parsedOpcode, len(scripts))
	for i, scr := range scripts {
		if len(scr) > maxScriptSize {
			return nil, ErrScriptTooBig
		}
		var err error
		vm.scripts[i], err = parseScript(scr)
		if err != nil {
			return nil, err
		}
	}

	if isMASSWitnessProgram(vm.scripts[0]) || isWitnessProgram(vm.scripts[0]) {
		var err error
		vm.witnessVersion, vm.witnessProgram, vm.witnessExtProg, err = extractWitnessProgramInfo(vm.scripts[0])
		if err != nil {
			return nil, err
		}
		if vm.witnessProgram == nil && len(tx.TxIn[txIdx].Witness) != 0 {
			return nil, ErrWitnessUnexpected
		}
	} /* else {
		return nil, ErrwitnessProgramFormat
	} */

	// // The engine stores the scripts in parsed form using a slice.  This
	// // allows multiple scripts to be executed in sequence.  For example,
	// // with a pay-to-script-hash transaction, there will be ultimately be
	// // a third script to execute.

	// if isWitnessScriptHash(pops) {
	// 	// var witProgram []byte
	// 	// witProgram = pkScript
	// 	// if witProgram != nil {
	// 	// 	var err error
	// 	// 	vm.witnessVersion, vm.witnessProgram, err = ExtractWitnessProgramInfo(witProgram)
	// 	// 	if err != nil {
	// 	// 		return nil, err
	// 	// 	}
	// 	// } else {
	// 	// 	// If we didn't find a witness program in either the
	// 	// 	// pkScript or as a datapush within the sigScript, then
	// 	// 	// there MUST NOT be any witness data associated with
	// 	// 	// the input being validated.
	// 	// 	if vm.witnessProgram == nil && len(tx.TxIn[txIdx].Witness) != 0 {
	// 	// 		return nil, ErrWitnessUnexpected
	// 	// 	}
	// 	// }
	// 	vm.witnessVersion, vm.witnessProgram, err = extractWitnessProgramInfo(pops)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	witnessHash := massutil.Hash160(witnessRedeemScript)
	// 	if !bytes.Equal(witnessHash, vm.witnessProgram) {
	// 		return nil, ErrWitnessProgramMismatch
	// 	}
	// } else if isWitnessStakingScript(pops) {
	// 	x := binary.LittleEndian.Uint64(pops[0].data)
	// 	if !wire.IsValidFrozenPeriod(x) {
	// 		return nil, ErrLockValue
	// 	}
	// 	vm.witnessVersion = 10
	// 	vm.witnessProgram = pkScript
	// 	vm.savedFirstStack = tx.TxIn[txIdx].Witness
	// } else if isWitnessBindingScript(pops) {
	// 	vm.witnessVersion = asSmallInt(pops[0].opcode)
	// 	vm.witnessProgram = pops[1].data
	// 	witnessHash := massutil.Hash160(witnessRedeemScript)
	// 	if !bytes.Equal(witnessHash, vm.witnessProgram) {
	// 		return nil, ErrWitnessProgramMismatch
	// 	}
	// } else {
	// 	return nil, errors.New("nonstandard pkScript")
	// }
	// scripts := [][]byte{witnessSig, witnessRedeemScript}
	// vm.scripts = make([][]parsedOpcode, len(scripts))
	// for i, scr := range scripts {
	// 	if len(scr) > maxScriptSize {
	// 		return nil, ErrStackElementTooBig
	// 	}
	// 	var err error
	// 	vm.scripts[i], err = parseScript(scr)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }
	vm.tx = *tx
	vm.txIdx = txIdx

	return &vm, nil
}
