package ngtypes

// pre-defined extra formats for the smart contracts

import (
	"bytes"
	"compress/flate"
	"io"

	"github.com/pkg/errors"

	"github.com/ngchain/ngcore/utils"
)

var ErrCommitExtraInvalid = errors.New("malformed commit extra payload")

// A CommitTx's extra carries the WHOLE contract module — a full
// snapshot, like a git commit stores a blob, not a diff. Diffing made
// sense when contracts were hand-written text; compiled wasm relayouts
// entirely on any change, so a "patch" would be as big as the module.
// The bytes are deflate-compressed with a one-byte format tag.
const (
	commitRaw     byte = 0x00 // the module bytes, uncompressed
	commitDeflate byte = 0x01 // flate(module)
)

// EncodeCommitCode compresses a contract module into a commit extra
func EncodeCommitCode(code []byte) []byte {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err == nil {
		if _, err = w.Write(code); err == nil {
			err = w.Close()
		}
	}

	if err == nil && buf.Len() < len(code) {
		return append([]byte{commitDeflate}, buf.Bytes()...)
	}

	return append([]byte{commitRaw}, code...)
}

// DecodeCommitCode recovers the contract module from a commit extra
func DecodeCommitCode(extra []byte) ([]byte, error) {
	if len(extra) < 1 {
		return nil, ErrCommitExtraInvalid
	}

	switch extra[0] {
	case commitRaw:
		return extra[1:], nil
	case commitDeflate:
		r := flate.NewReader(bytes.NewReader(extra[1:]))
		defer r.Close()

		// bound the inflated size to the source cap area (the state
		// layer enforces the exact MaxContractSourceSize); this just
		// stops a decompression bomb during decode
		return io.ReadAll(io.LimitReader(r, 1<<24))
	default:
		return nil, ErrCommitExtraInvalid
	}
}

// CallSelector returns the eth-style 4-byte selector of an entry name
func CallSelector(entry string) []byte {
	return utils.KeccakSum256([]byte(entry))[:4]
}

// EncodeCallData packs an entry selector and its args into a tx extra
func EncodeCallData(entry string, args []byte) []byte {
	if entry == "" || entry == "main" {
		if len(args) == 0 {
			return []byte{}
		}
		entry = "main"
	}

	out := make([]byte, 0, 4+len(args))
	out = append(out, CallSelector(entry)...)
	return append(out, args...)
}
