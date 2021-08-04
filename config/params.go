package config

import (
	"errors"
	"math/big"
	"strings"

	"github.com/wangxinyu2018/mass-core/consensus"
	"github.com/wangxinyu2018/mass-core/wire"
)

var (
	// bigOne is 1 represented as a big.Int.  It is defined here to avoid
	// the overhead of creating it multiple times.
	bigOne = big.NewInt(1)

	// mainPocLimit is the smallest proof of capacity target.
	mainPocLimit = new(big.Int).Sub(new(big.Int).Lsh(bigOne, 20), bigOne)
)

var (
	// ErrUnknownHDKeyID describes an error where the provided id which
	// is intended to identify the network for a hierarchical deterministic
	// private extended key is not registered.
	ErrUnknownHDKeyID = errors.New("unknown hd private extended key bytes")
)

var (
	pubKeyHashAddrIDs    = make(map[byte]struct{})
	scriptHashAddrIDs    = make(map[byte]struct{})
	bech32SegwitPrefixes = make(map[string]struct{})
	hdPrivToPubKeyIDs    = make(map[[4]byte][]byte)
)

// Register registers the network parameters for a Mass network.  This may
// error with ErrDuplicateNet if the network is already registered (either
// due to a previous Register call, or the network being one of the default
// networks).
//
// Network parameters should be registered into this package by a main package
// as early as possible.  Then, library packages may lookup networks or network
// parameters based on inputs and work regardless of the network being standard
// or not.
func Register(params *Params) error {
	pubKeyHashAddrIDs[params.PubKeyHashAddrID] = struct{}{}
	scriptHashAddrIDs[params.ScriptHashAddrID] = struct{}{}
	hdPrivToPubKeyIDs[params.HDPrivateKeyID] = params.HDPublicKeyID[:]

	// A valid Bech32 encoded segwit address always has as prefix the
	// human-readable part for the given net followed by '1'.
	bech32SegwitPrefixes[params.Bech32HRPSegwit+"1"] = struct{}{}
	return nil
}

// Checkpoint identifies a known good point in the block chain.  Using
// checkpoints allows a few optimizations for old blocks during initial download
// and also prevents forks from old blocks.
//
// Each checkpoint is selected based upon several factors.  See the
// documentation for blockchain.IsCheckpointCandidate for details on the
// selection criteria.
type Checkpoint struct {
	Height uint64
	Hash   *wire.Hash
}

// Params defines a Mass network by its parameters.  These parameters may be
// used by Mass applications to differentiate networks as well as addresses
// and keys for one network from those intended for use on another network.
type Params struct {
	Name        string
	DefaultPort string
	DNSSeeds    []string

	// Chain parameters
	GenesisBlock           *wire.MsgBlock
	GenesisHash            *wire.Hash
	ChainID                *wire.Hash
	PocLimit               *big.Int
	SubsidyHalvingInterval uint64
	ResetMinDifficulty     bool

	// Checkpoints ordered from oldest to newest.
	Checkpoints []Checkpoint

	// Mempool parameters
	RelayNonStdTxs bool

	// Human-readable part for Bech32 encoded segwit addresses, as defined
	// in BIP 173.
	Bech32HRPSegwit string

	// Address encoding magics
	PubKeyHashAddrID        byte // First byte of a P2PKH address
	ScriptHashAddrID        byte // First byte of a P2SH address
	PrivateKeyID            byte // First byte of a WIF private key
	WitnessPubKeyHashAddrID byte // First byte of a P2WPKH address
	WitnessScriptHashAddrID byte // First byte of a P2WSH address

	// BIP32 hierarchical deterministic extended key magics
	HDPrivateKeyID [4]byte
	HDPublicKeyID  [4]byte

	// BIP44 coin type used in the hierarchical deterministic path for
	// address generation.
	HDCoinType uint32
}

// ChainParams defines the network parameters for the main Mass network.
var ChainParams = Params{
	Name:        defaultChainTag,
	DefaultPort: "43453",
	DNSSeeds:    []string{},

	// Chain parameters
	GenesisBlock:           &genesisBlock,
	ChainID:                &genesisChainID,
	PocLimit:               mainPocLimit,
	SubsidyHalvingInterval: consensus.SubsidyHalvingInterval,
	ResetMinDifficulty:     false,

	// Checkpoints ordered from oldest to newest.
	Checkpoints: []Checkpoint{
		{150000, newHashFromStr("b5864e69b3639fde4dbe2496c9f9f5164591917082eb95c0d99022513286b0f2")},
		{300000, newHashFromStr("ae09d31fa05a1df7eb4e81fcf14db533d64533b134f96f9afa5a2ab5cfb086c5")},
		{450000, newHashFromStr("677e150515e016e04d7fb3532a69fc99a27a74acb10819827a015a3f9fe19542")},
		{600000, newHashFromStr("bb10bd5a7ab626b61eefb387d906e76ea69fb08912fbc0cb031e489157381b81")},
		{750000, newHashFromStr("669c2fae2e772623d985f364478fff25bb4a39d9d986493d8236f01e8abab78d")},
		{900000, newHashFromStr("1b78fbfc8e7b3a0bcdd82ab16da3a5136d974bc51b108519584b08f1d879334c")},
		{1050000, newHashFromStr("8c929a910184a8558cf40a11b887582bd08e0b6a2814b291ccc891423466f9cd")},
		{1200000, newHashFromStr("965fd52574178734be81c1d75d632090e5bfa3607d35e4d27487fccff993c042")},
		{1390000, newHashFromStr("bd02ce24fa5dbf6354a19def8e1de746a832bf904e4f9421d7355d166e8acf79")},
	},

	// Mempool parameters
	RelayNonStdTxs: false,

	// Human-readable part for Bech32 encoded segwit addresses, as defined in
	// BIP 173.
	Bech32HRPSegwit: "ms", // always ms for main net

	// Address encoding magics
	PubKeyHashAddrID:        0x00, // starts with 1
	ScriptHashAddrID:        0x05, // starts with 3
	PrivateKeyID:            0x80, // starts with 5 (uncompressed) or K (compressed)
	WitnessPubKeyHashAddrID: 0x06, // starts with p2
	WitnessScriptHashAddrID: 0x0A, // starts with 7Xh

	// BIP32 hierarchical deterministic extended key magics
	HDPrivateKeyID: [4]byte{0x04, 0x88, 0xad, 0xe4}, // starts with xprv
	HDPublicKeyID:  [4]byte{0x04, 0x88, 0xb2, 0x1e}, // starts with xpub

	// BIP44 coin type used in the hierarchical deterministic path for
	// address generation.
	HDCoinType: HDCoinTypeMassMainNet,
}

// IsPubKeyHashAddrID returns whether the id is an identifier known to prefix a
// pay-to-pubkey-hash address on any default or registered network.  This is
// used when decoding an wallet string into a specific wallet type.  It is up
// to the caller to check both this and IsScriptHashAddrID and decide whether an
// wallet is a pubkey hash wallet, script hash wallet, neither, or
// undeterminable (if both return true).
func IsPubKeyHashAddrID(id byte) bool {
	_, ok := pubKeyHashAddrIDs[id]
	return ok
}

// IsScriptHashAddrID returns whether the id is an identifier known to prefix a
// pay-to-script-hash address on any default or registered network.  This is
// used when decoding an wallet string into a specific wallet type.  It is up
// to the caller to check both this and IsPubKeyHashAddrID and decide whether an
// wallet is a pubkey hash wallet, script hash wallet, neither, or
// undeterminable (if both return true).
func IsScriptHashAddrID(id byte) bool {
	_, ok := scriptHashAddrIDs[id]
	return ok
}

// HDPrivateKeyToPublicKeyID accepts a private hierarchical deterministic
// extended key id and returns the associated public key id.  When the provided
// id is not registered, the ErrUnknownHDKeyID error will be returned.
func HDPrivateKeyToPublicKeyID(id []byte) ([]byte, error) {
	if len(id) != 4 {
		return nil, ErrUnknownHDKeyID
	}

	var key [4]byte
	copy(key[:], id)
	pubBytes, ok := hdPrivToPubKeyIDs[key]
	if !ok {
		return nil, ErrUnknownHDKeyID
	}

	return pubBytes, nil
}

// IsBech32SegwitPrefix returns whether the prefix is a known prefix for segwit
// addresses on any default or registered network.  This is used when decoding
// an wallet string into a specific wallet type.
func IsBech32SegwitPrefix(prefix string) bool {
	prefix = strings.ToLower(prefix)
	_, ok := bech32SegwitPrefixes[prefix]
	return ok
}

// Must call this func when mock chain.
func UpdateGenesisBlock(blk *wire.MsgBlock) {
	ChainParams.GenesisBlock = blk
	// update ChainID
	chainID, err := ChainParams.GenesisBlock.Header.GetChainID()
	if err != nil {
		panic(err) // should not happen
	}
	ChainParams.ChainID = &chainID
	genesisChainID = chainID
	genesisHeader.ChainID = chainID
	ChainParams.GenesisBlock.Header.ChainID = chainID
	//update Block Hash
	genesisHash = genesisHeader.BlockHash()
	ChainParams.GenesisHash = &genesisHash
}

func init() {
	// update genesis block
	UpdateGenesisBlock(ChainParams.GenesisBlock)
	// register chainParams
	Register(&ChainParams)
}

func newHashFromStr(hexStr string) *wire.Hash {
	hash, err := wire.NewHashFromStr(hexStr)
	if err != nil {
		panic(err)
	}
	return hash
}
