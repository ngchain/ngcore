package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type Job struct {
	*ngtypes.FullBlock
	RawHeader string
	Nonce     []byte
}

func NewJob(rawHeader string) *Job {
	b, err := hex.DecodeString(rawHeader)
	if err != nil {
		panic(err)
	}
	raw := make([]byte, 153)
	copy(raw, b)
	block := ngtypes.NewBlock(
		ngtypes.Network(raw[0]),
		binary.LittleEndian.Uint64(raw[1:9][:]),
		binary.LittleEndian.Uint64(raw[9:17]),
		raw[17:49],
		raw[49:81],
		raw[81:113],
		bytes.TrimLeft(utils.ReverseBytes(raw[113:145]), string(byte(0))), // remove left padding
		raw[145:153],
		nil,
		nil,
	)

	return &Job{
		RawHeader: rawHeader,
		FullBlock: block,
		Nonce:     nil,
	}
}

func (j *Job) SetNonce(nonce []byte) {
	j.Nonce = nonce
}
