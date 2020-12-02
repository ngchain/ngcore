package consensus

import (
	"bytes"
	"math/big"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/ngchain/ngcore/ngtypes"
)

type remoteRecord struct {
	id                   peer.ID
	origin               uint64 // rank
	latest               uint64
	checkpointHash       []byte   // trigger
	checkpointActualDiff *big.Int // rank
	lastChatTime         int64

	failureNum     int
	lastFailedTime int64
}

// RULE: when forking?
// Situation #1: remote height is higher than local, AND checkpoint is on higher level
// Situation #2: remote height is higher than local, AND checkpoint is on same level, AND remote checkpoint takes more rank (with more ActualDiff)
// TODO: add a cap for forking
func (r *remoteRecord) shouldFork(latestCheckPoint *ngtypes.Block, latestHeight uint64) bool {
	if time.Now().Unix() < r.lastFailedTime+int64(60*60) {
		return false
	}

	cpHash := latestCheckPoint.Hash()

	if !bytes.Equal(r.checkpointHash, cpHash) &&
		r.latest > latestHeight &&
		r.latest/ngtypes.BlockCheckRound > latestHeight/ngtypes.BlockCheckRound {
		return true
	}

	if !bytes.Equal(r.checkpointHash, cpHash) &&
		r.latest > latestHeight &&
		r.latest/ngtypes.BlockCheckRound == latestHeight/ngtypes.BlockCheckRound &&
		r.checkpointActualDiff != nil &&
		r.checkpointActualDiff.Cmp(latestCheckPoint.GetActualDiff()) > 0 {
		return true
	}

	return false
}

func (r *remoteRecord) recordFailure() {
	r.failureNum++
	if r.failureNum > 3 {
		r.lastFailedTime = time.Now().Unix()
	}
}
