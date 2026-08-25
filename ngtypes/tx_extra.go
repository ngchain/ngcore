package ngtypes

// pre-defined extra formats for the smart contracts

import (
	"bytes"
	"compress/flate"
	"io"

	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"
)

var ErrCommitExtraInvalid = errors.New("malformed commit extra payload")

// A DeployTx's extra carries the WHOLE contract module — a full
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

// CallData is a contract call's payload: the export to run and its raw
// argument bytes, RLP-encoded into a tx's Extra (and into the calldata a
// contract hands to contract.call). ngcore dispatches by the export NAME
// directly — a wasm module already has named exports, so there is no
// eth-style 4-byte selector, and thus no selector-collision class to
// guard against. An empty Method addresses the default "main" entry.
type CallData struct {
	Method string
	Args   []byte
}

// EncodeCallData RLP-encodes a contract call payload into a tx extra. A
// "main"/empty method plus empty args yields an empty extra — a bare
// value transfer carries no calldata
func EncodeCallData(method string, args []byte) []byte {
	if method == "main" {
		method = "" // the default entry is addressed by the empty method
	}
	if method == "" && len(args) == 0 {
		return []byte{}
	}

	out, err := rlp.EncodeToBytes(&CallData{Method: method, Args: args})
	if err != nil {
		return nil
	}

	return out
}

// DecodeCallData recovers a contract call payload from a tx extra
func DecodeCallData(extra []byte) (method string, args []byte, err error) {
	var cd CallData
	if err := rlp.DecodeBytes(extra, &cd); err != nil {
		return "", nil, err
	}

	return cd.Method, cd.Args, nil
}
