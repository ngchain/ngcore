package consensus

import (
	"bytes"
	"math/big"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/atomic"

	"github.com/ngchain/ngcore/ngtypes"
)

type RemoteRecord struct {
	id     peer.ID
	origin uint64 // rank
	latest uint64

	checkpointHeight     uint64
	checkpointHash       []byte   // trigger
	checkpointActualDiff *big.Int // rank
	lastChatTime         int64

	failureNum     *atomic.Uint32
	lastFailedTime int64
}

func NewRemoteRecord(id peer.ID, origin, latest uint64, checkpointHash, checkpointActualDiff []byte) *RemoteRecord {
	var checkpointHeight uint64
	if latest%ngtypes.BlockCheckRound == 0 {
		checkpointHeight = latest
	} else {
		checkpointHeight = latest - latest%ngtypes.BlockCheckRound
	}

	return &RemoteRecord{
		id:                   id,
		origin:               origin,
		latest:               latest,
		checkpointHeight:     checkpointHeight,
		checkpointHash:       checkpointHash,
		checkpointActualDiff: new(big.Int).SetBytes(checkpointActualDiff),
		lastChatTime:         time.Now().Unix(),
		failureNum:           atomic.NewUint32(0),
		lastFailedTime:       0,
	}
}

func (r *RemoteRecord) update(origin, latest uint64, checkpointHash, checkpointActualDiff []byte) {
	r.origin = origin
	r.latest = latest
	r.checkpointHeight = latest - latest%ngtypes.BlockCheckRound
	r.checkpointHash = checkpointHash
	r.checkpointActualDiff = new(big.Int).SetBytes(checkpointActualDiff)
	r.lastChatTime = time.Now().Unix()
}

// syncRetryCooldown is how long a peer is skipped after repeated sync/converge
// failures. The old value (1h) was catastrophic on a 1s-block chain: a node
// that momentarily failed stayed hours behind. Keep it short so a laggard
// retries quickly; recordSuccess resets the failure counter entirely.
const syncRetryCooldown = 15 * time.Second

func (r *RemoteRecord) shouldSync(latestHeight uint64) bool {
	if time.Now().Unix() < r.lastFailedTime+int64(syncRetryCooldown.Seconds()) {
		return false
	}

	if r.latest/ngtypes.BlockCheckRound <= latestHeight/ngtypes.BlockCheckRound {
		return false
	}

	return true
}

// RULE: when converging?
// Situation #1: remote height is higher than local, AND checkpoint is on higher level
// Situation #2: remote height is higher than local, AND checkpoint is on same level, AND remote checkpoint takes more rank (with more ActualDiff)
// TODO: add a cap for converging.
func (r *RemoteRecord) shouldConverge(latestCheckPoint *ngtypes.FullBlock, latestHeight uint64) bool {
	if time.Now().Unix() < r.lastFailedTime+int64(syncRetryCooldown.Seconds()) {
		return false
	}

	cpHash := latestCheckPoint.GetHash()

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

func (r *RemoteRecord) recordFailure() {
	r.failureNum.Inc()
	if r.failureNum.Load() > 3 {
		r.lastFailedTime = time.Now().Unix()
	}
}

// recordSuccess clears the failure state so a peer that recovers is no longer
// penalized by past failures (otherwise failureNum only ever grew, eventually
// pinning every peer into the cooldown branch permanently)
func (r *RemoteRecord) recordSuccess() {
	r.failureNum.Store(0)
	r.lastFailedTime = 0
}
