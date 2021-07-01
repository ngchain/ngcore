package wired

import "github.com/ngchain/ngcore/ngtypes"

// StatusPayload is the payload used when ping pong.
type StatusPayload struct {
	Origin         uint64
	Latest         uint64
	CheckpointHash []byte
	CheckpointDiff []byte // actual diff
}

type GetChainPayload struct {
	Type ChainType
	From [][]byte
	To   []byte
}

type ChainPayload struct {
	Type    ChainType
	Headers []*ngtypes.BlockHeader `rlp:"optional"`
	Blocks  []*ngtypes.Block       `rlp:"optional"`
}

// GetSheetPayload is the payload for getting a sheet from remote
// Design:
// fast-sync for state
// support checkpoint height only
// when fast-sync:
//   1. sync to latest checkpoint
//   2. sync to latest checkpoint state
//   3. sync the remaining blocks
//   4. update local state
type GetSheetPayload struct {
	Height uint64
	Hash   []byte
}

type SheetPayload struct {
	Sheet *ngtypes.Sheet
}
