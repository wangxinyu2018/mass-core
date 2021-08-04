package template_data

import (
	"bufio"
	"encoding/hex"
	"math/big"
	"os"
	"testing"

	"github.com/wangxinyu2018/mass-core/blockchain"
	"github.com/wangxinyu2018/mass-core/massutil"
	"github.com/wangxinyu2018/mass-core/wire"
)

func Test_CheckData(t *testing.T) {
	file, err := os.Open("block.dat")
	if err != nil {
		t.Fatalf("failed to open file, %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		buf, err := hex.DecodeString(scanner.Text())
		if err != nil {
			t.Fatalf("failed to read line, %v", err)
		}
		block, err := massutil.NewBlockFromBytes(buf, wire.Packet)
		if err != nil {
			t.Fatalf("failed to new block from bytes, %v", err)
		}
		err = blockchain.CheckProofOfCapacity(block, big.NewInt(0))
		if err != nil {
			t.Fatalf("failed to check proof, %v", err)
		}
		t.Logf("check proof pass, %v", block.Height())
	}
}
