package ngtypes

// LightWeightBlock is a light weight block strcut for db storage
type LightWeightBlock struct {
	Header *BlockHeader
	Txs    [][]byte // tx hashes
	Subs   [][]byte // subblock hashes
}
