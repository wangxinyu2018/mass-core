package poc_test

import (
	"encoding/hex"
	"math/big"
	"math/rand"
	"reflect"
	"testing"

	"github.com/ylpool/mass-core/poc"
	"github.com/ylpool/mass-core/poc/pocutil"
	"github.com/ylpool/mass-core/pocec"
)

func TestEnsureBitLength(t *testing.T) {
	tests := []*struct {
		bl    int
		valid bool
	}{
		{
			bl:    22,
			valid: false,
		},
		{
			bl:    23,
			valid: false,
		},
		{
			bl:    24,
			valid: true,
		},
		{
			bl:    25,
			valid: false,
		},
		{
			bl:    39,
			valid: false,
		},
		{
			bl:    40,
			valid: true,
		},
		{
			bl:    41,
			valid: false,
		},
		{
			bl:    42,
			valid: false,
		},
	}

	for i, test := range tests {
		if valid := poc.ProofTypeDefault.EnsureBitLength(test.bl); valid != test.valid {
			t.Errorf("%d, EnsureBitLength not equal, got = %v, want = %v", i, valid, test.valid)
		}
	}
}

func TestProof_Encode(t *testing.T) {
	var BL = 24
	var endBL = 40

	for bl := BL; bl < endBL; bl += 2 {
		x := rand.Intn(int(1) << uint(bl))
		xPrime := rand.Intn(int(1) << uint(bl))
		proof := &poc.DefaultProof{
			X:      pocutil.PoCValue2Bytes(pocutil.PoCValue(x), bl),
			XPrime: pocutil.PoCValue2Bytes(pocutil.PoCValue(xPrime), bl),
			BL:     bl,
		}
		data := proof.Encode()
		decoded := new(poc.DefaultProof)
		err := decoded.Decode(data)
		if err != nil {
			t.Fatalf("decode fail, X = %d, XPrime = %d, BitLength = %d, Encoded = %s",
				x, xPrime, bl, hex.EncodeToString(data))
		}
		if !reflect.DeepEqual(decoded, proof) {
			t.Fatalf("proof encode/decode not equal to original, X = %d, XPrime = %d, BitLength = %d, Encoded = %s",
				x, xPrime, bl, hex.EncodeToString(data))
		}
	}
}

func TestProof_Decode(t *testing.T) {
	var BL = 24
	var endBL = 40

	for bl := BL; bl < endBL; bl += 2 {
		randLength := rand.Intn(20)
		data := make([]byte, randLength)

		err := new(poc.DefaultProof).Decode(data)
		if len(data) != 17 && err != poc.ErrProofDecodeDataSize {
			t.Fatalf("decode fail, BitLength = %d, Encoded = %s",
				bl, hex.EncodeToString(data))
		}
	}
}

func TestVerifyProof(t *testing.T) {
	challenge, err := pocutil.DecodeStringToHash("f17a8b5534fb1a9d34c831d0766fbc77b0b718500412c6647f48fda0dd8fa780")
	if err != nil {
		t.Fatal(err)
	}
	pkByte, _ := hex.DecodeString("02be7ff1bbbd42b808cb6b7de2d22cd53dea771c9c599fb034c7b15bae0ec53eb3")
	pk, err := pocec.ParsePubKey(pkByte, pocec.S256())
	if err != nil {
		t.Fatal(err)
	}
	pubKeyHash := pocutil.PubKeyHash(pk)

	tests := []struct {
		proof poc.Proof
		err   error
	}{
		{
			proof: &poc.DefaultProof{
				X:      []byte{0xeb, 0xd0, 0x8b, 0xeb},
				XPrime: []byte{0x98, 0x87, 0x63, 0x0a},
				BL:     32,
			},
			err: nil,
		},
		{
			proof: &poc.DefaultProof{
				X:      []byte{0xeb, 0xd0, 0x8b, 0xeb},
				XPrime: []byte{0x98, 0x87, 0x63, 0x0a},
				BL:     22,
			},
			err: poc.ErrProofInvalidBitLength,
		},
		{
			proof: &poc.DefaultProof{
				X:      []byte{0xeb, 0xd0, 0x8b, 0xeb},
				XPrime: []byte{0x98, 0x87, 0x63, 0x0a},
				BL:     42,
			},
			err: poc.ErrProofInvalidBitLength,
		},
		{
			proof: &poc.DefaultProof{
				X:      []byte{0xeb, 0xd0, 0x8b, 0xea},
				XPrime: []byte{0x98, 0x87, 0x63, 0x0a},
				BL:     32,
			},
			err: poc.ErrProofInvalidFlipValue,
		},
		{
			proof: &poc.DefaultProof{
				X:      []byte{0x08, 0xd5, 0x57, 0xc3},
				XPrime: []byte{0xe5, 0xf5, 0x54, 0xcd},
				BL:     32,
			},
			err: poc.ErrProofInvalidChallenge,
		},
	}

	for i, test := range tests {
		if err := poc.VerifyProof(test.proof, pubKeyHash, challenge, false); err != test.err {
			t.Errorf("%d, error not matched, got = %v, want = %v", i, err, test.err)
		}
	}

}

func TestProof_GetQuality(t *testing.T) {
	tests := []*struct {
		proof        poc.Proof
		slot, height uint64
		quality      *big.Int
	}{
		{
			proof: &poc.DefaultProof{
				X:      []byte{0xac, 0xc5, 0x99, 0x96},
				XPrime: []byte{0x94, 0x4f, 0x01, 0x16},
				BL:     32,
			},
			slot: 522439786, height: 0,
			quality: big.NewInt(2406673284404964),
		},
	}

	for i, test := range tests {
		if quality := test.proof.Quality(test.slot, test.height); quality.Cmp(test.quality) != 0 {
			t.Errorf("%d, GetQuality not equal, got = %d, want = %d", i, quality, test.quality)
		}
	}
}

func TestProof_GetVerifiedQuality(t *testing.T) {
	challenge, err := pocutil.DecodeStringToHash("f17a8b5534fb1a9d34c831d0766fbc77b0b718500412c6647f48fda0dd8fa780")
	if err != nil {
		t.Fatal(err)
	}
	pkByte, _ := hex.DecodeString("02be7ff1bbbd42b808cb6b7de2d22cd53dea771c9c599fb034c7b15bae0ec53eb3")
	pk, err := pocec.ParsePubKey(pkByte, pocec.S256())
	if err != nil {
		t.Fatal(err)
	}
	pubKeyHash := pocutil.PubKeyHash(pk)

	tests := []*struct {
		proof        poc.Proof
		slot, height uint64
		quality      *big.Int
		err          error
	}{
		{
			proof: &poc.DefaultProof{
				X:      []byte{0xeb, 0xd0, 0x8b, 0xeb},
				XPrime: []byte{0x98, 0x87, 0x63, 0x0a},
				BL:     32,
			},
			slot: 522439821, height: 1,
			quality: big.NewInt(319231376303788),
			err:     nil,
		},
		{
			proof: &poc.DefaultProof{
				X:      []byte{0xeb, 0xd0, 0x8b, 0xea},
				XPrime: []byte{0x98, 0x87, 0x63, 0x0a},
				BL:     32,
			},
			slot: 522439821, height: 1,
			quality: nil,
			err:     poc.ErrProofInvalidFlipValue,
		},
	}

	for i, test := range tests {
		if quality, err := test.proof.VerifiedQuality(pubKeyHash, challenge, false, test.slot, test.height); err != test.err {
			t.Errorf("%d, GetVerifiedQuality error not matched, got = %d, want = %d", i, err, test.err)
		} else if err == nil && quality.Cmp(test.quality) != 0 {
			t.Errorf("%d, GetVerifiedQuality not equal, got = %d, want = %d", i, quality, test.quality)
		}
	}
}
