package blockchain

import (
	"math/big"
	"time"

	"github.com/wangxinyu2018/mass-core/config"
	"github.com/wangxinyu2018/mass-core/consensus/challenge"
	"github.com/wangxinyu2018/mass-core/consensus/difficulty"
	"github.com/wangxinyu2018/mass-core/wire"
)

// CalcNextChallenge calculates the required challenge for the block
// after the end of the current best chain based on the challenge adjustment
// rules.
func (chain *Blockchain) CalcNextChallenge() (*wire.Hash, error) {
	return calcNextChallenge(chain.blockTree.bestBlockNode())
}

func calcNextChallenge(lastNode *BlockNode) (*wire.Hash, error) {
	var size uint64 = challenge.MaxReferredBlocks
	if lastNode.Height+1 < size {
		size = lastNode.Height + 1
	}
	headers := make([]*wire.BlockHeader, size)
	refNode := lastNode
	for i := uint64(0); i < size; i++ {
		headers[i] = refNode.BlockHeader()
		refNode = refNode.Parent
	}
	return challenge.CalcNextChallenge(lastNode.BlockHeader(), headers)
}

func (chain *Blockchain) CalcNextTarget(newBlockTime time.Time) (*big.Int, error) {
	return calcNextTarget(chain.blockTree.bestBlockNode(), newBlockTime, chain.chainParams)
}

func calcNextTarget(lastNode *BlockNode, newBlockTime time.Time, chainParams *config.Params) (*big.Int, error) {
	return difficulty.CalcNextRequiredDifficulty(lastNode.BlockHeader(), newBlockTime, chainParams)
}
