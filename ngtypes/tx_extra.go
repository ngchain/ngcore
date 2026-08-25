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

// Reserved contract entry-export names. The protocol BINDS to these four
// exports and calls them itself (the tx handler, the deploy hook, the UUPS
// upgrade authorizer, the account-abstraction gate) — a contract never
// dispatches to them as ordinary methods. The "ng:" namespace prefix keeps
// them from colliding with a developer's own exported methods: a colon is
// not a valid identifier character, so a plain `#[no_mangle] fn` (or an
// `(export "...")` a dev writes for business logic) cannot emit one by
// accident — these can only be declared with deliberate intent
// (e.g. `#[export_name = "ng:validate"]`). ngstate mirrors these into its
// VMEntryOn* constants; keep the two in lockstep.
const (
	EntryOnTx       = "ng:main"     // run on a transact tx (the default entry)
	EntryOnActivate = "ng:init"     // run once when a deploy goes live
	EntryOnUpgrade  = "ng:upgrade"  // must authorize a UUPS code replacement
	EntryOnValidate = "ng:validate" // gates every tx FROM the account (AA)
)

// CallData is a contract call's payload: the export to run and its raw
// argument bytes, RLP-encoded into a tx's Extra (and into the calldata a
// contract hands to contract.call). ngcore dispatches by the export NAME
// directly — a wasm module already has named exports, so there is no
// eth-style 4-byte selector, and thus no selector-collision class to
// guard against. An empty Method addresses the default entry (EntryOnTx).
type CallData struct {
	Method string
	Args   []byte
}

// EncodeCallData RLP-encodes a contract call payload into a tx extra. Naming
// the default entry (EntryOnTx) or the empty method, both with empty args,
// yields an empty extra — a bare value transfer carries no calldata
func EncodeCallData(method string, args []byte) []byte {
	if method == EntryOnTx {
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
