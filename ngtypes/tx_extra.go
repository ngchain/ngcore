package ngtypes

// some pre-defined extra formats for the smart contracts

import (
	"bytes"
	"compress/flate"
	"io"

	"github.com/c0mm4nd/rlp"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
)

// Hunk is one replacement in the contract text at Pos (a byte offset in
// the ORIGINAL text).
//
// It comes in two shapes depending on the enclosing EditExtra:
//   - content shape (no BaseHash): Del carries the removed bytes and is
//     verified against the on-chain text; DelLen must be 0
//   - hashed shape (with BaseHash): only DelLen is carried — the whole
//     original text is pinned by BaseHash instead, which is cheaper as
//     soon as the removed content outgrows one hash
type Hunk struct {
	Pos    uint64
	DelLen uint64
	Del    []byte
	Ins    []byte
}

// EditExtra is the payload of an Edit Tx: a whole patch applied
// atomically. Hunks use original-text coordinates, must be sorted
// ascending and must not overlap
type EditExtra struct {
	BaseHash []byte // optional sha3-256 of the original text
	Hunks    []Hunk
}

var (
	ErrHunkNone         = errors.New("edit contains no effective hunk")
	ErrHunkOutOfBound   = errors.New("hunk is out of the text bound")
	ErrHunkOverlap      = errors.New("hunks overlap or are not sorted")
	ErrHunkMismatch     = errors.New("hunk Del does not match the original text")
	ErrHunkShapeInvalid = errors.New("hunk shape does not match the patch mode")
	ErrBaseMismatch     = errors.New("patch base hash does not match the original text")
	ErrEditExtraInvalid = errors.New("malformed edit extra payload")
)

// NewEditExtra wraps the hunks into the smaller patch shape: when the
// removed content outweighs one hash, the Del bytes are dropped and the
// original text is pinned by its sha3-256 instead
func NewEditExtra(baseText []byte, hunks []Hunk) *EditExtra {
	totalDel := 0
	for _, h := range hunks {
		totalDel += len(h.Del)
	}
	if totalDel <= HashSize {
		return &EditExtra{Hunks: hunks}
	}

	hashed := make([]Hunk, len(hunks))
	for i, h := range hunks {
		hashed[i] = Hunk{Pos: h.Pos, DelLen: uint64(len(h.Del)), Ins: h.Ins}
	}
	baseHash := sha3.Sum256(baseText)

	return &EditExtra{BaseHash: baseHash[:], Hunks: hashed}
}

// Apply applies the patch onto text, returning the new text.
// The input text is never mutated; any invalid hunk fails the whole patch
func (x *EditExtra) Apply(text []byte) ([]byte, error) {
	if len(x.Hunks) == 0 {
		return nil, ErrHunkNone
	}

	pinned := len(x.BaseHash) != 0
	if pinned {
		if len(x.BaseHash) != HashSize {
			return nil, ErrEditExtraInvalid
		}
		baseHash := sha3.Sum256(text)
		if !bytes.Equal(baseHash[:], x.BaseHash) {
			return nil, ErrBaseMismatch
		}
	}

	out := make([]byte, 0, len(text))
	cursor := uint64(0)

	for _, h := range x.Hunks {
		delLen := uint64(len(h.Del))
		if pinned {
			if len(h.Del) != 0 {
				return nil, ErrHunkShapeInvalid
			}
			delLen = h.DelLen
		} else if h.DelLen != 0 {
			return nil, ErrHunkShapeInvalid
		}

		if delLen == 0 && len(h.Ins) == 0 {
			return nil, ErrHunkNone
		}
		if h.Pos < cursor {
			return nil, ErrHunkOverlap
		}
		end := h.Pos + delLen
		if h.Pos > uint64(len(text)) || end > uint64(len(text)) {
			return nil, ErrHunkOutOfBound
		}
		if !pinned && !bytes.Equal(text[h.Pos:end], h.Del) {
			return nil, ErrHunkMismatch
		}

		out = append(out, text[cursor:h.Pos]...)
		out = append(out, h.Ins...)
		cursor = end
	}

	return append(out, text[cursor:]...), nil
}

// wire encoding of the edit extra: 1 flag byte + payload
const (
	editExtraRaw     byte = 0x00 // rlp(EditExtra)
	editExtraDeflate byte = 0x01 // flate(rlp(EditExtra))
)

// Encode serializes the patch for the tx Extra field, compressing the
// payload when that actually shrinks it (large deploys compress well,
// tiny patches stay raw)
func (x *EditExtra) Encode() ([]byte, error) {
	raw, err := rlp.EncodeToBytes(x)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	if buf.Len() < len(raw) {
		return append([]byte{editExtraDeflate}, buf.Bytes()...), nil
	}

	return append([]byte{editExtraRaw}, raw...), nil
}

// DecodeEditExtra parses a tx Extra payload produced by Encode. The
// decompressed size is capped by TxMaxExtraSize against zip bombs
func DecodeEditExtra(data []byte) (*EditExtra, error) {
	if len(data) < 2 {
		return nil, ErrEditExtraInvalid
	}

	var raw []byte
	switch data[0] {
	case editExtraRaw:
		raw = data[1:]
	case editExtraDeflate:
		r := flate.NewReader(bytes.NewReader(data[1:]))
		decompressed, err := io.ReadAll(io.LimitReader(r, TxMaxExtraSize+1))
		if err != nil {
			return nil, errors.Wrap(ErrEditExtraInvalid, err.Error())
		}
		if len(decompressed) > TxMaxExtraSize {
			return nil, ErrTxExtraExcess
		}
		raw = decompressed
	default:
		return nil, ErrEditExtraInvalid
	}

	var x EditExtra
	if err := rlp.DecodeBytes(raw, &x); err != nil {
		return nil, errors.Wrap(ErrEditExtraInvalid, err.Error())
	}

	return &x, nil
}

// DiffHunks computes a minimal line-based patch turning oldText into
// newText. It is a plain LCS diff with per-hunk byte shrinking, so
// applying the patch always rebuilds newText exactly; identical texts
// yield no hunks. The returned hunks are in the content shape (Del
// carried); NewEditExtra picks the cheaper wire shape
func DiffHunks(oldText, newText []byte) []Hunk {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	// LCS table over lines
	n, m := len(oldLines), len(newLines)
	lcs := make([][]int32, n+1)
	for i := range lcs {
		lcs[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if bytes.Equal(oldLines[i], newLines[j]) {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var hunks []Hunk
	oldPos := uint64(0) // byte offset in oldText
	i, j := 0, 0
	for i < n || j < m {
		// common line: advance both
		if i < n && j < m && bytes.Equal(oldLines[i], newLines[j]) {
			oldPos += uint64(len(oldLines[i]))
			i++
			j++
			continue
		}

		// a run of differing lines forms one hunk
		hunk := Hunk{Pos: oldPos}
		for i < n || j < m {
			if i < n && j < m && bytes.Equal(oldLines[i], newLines[j]) {
				break
			}
			if j == m || (i < n && lcs[i+1][j] >= lcs[i][j+1]) {
				hunk.Del = append(hunk.Del, oldLines[i]...)
				oldPos += uint64(len(oldLines[i]))
				i++
			} else {
				hunk.Ins = append(hunk.Ins, newLines[j]...)
				j++
			}
		}
		hunks = append(hunks, shrinkHunk(hunk))
	}

	return hunks
}

// shrinkHunk trims the common prefix and suffix shared by Del and Ins,
// so a two-character tweak inside a long line patches just those bytes
func shrinkHunk(h Hunk) Hunk {
	p := 0
	for p < len(h.Del) && p < len(h.Ins) && h.Del[p] == h.Ins[p] {
		p++
	}

	s := 0
	for s < len(h.Del)-p && s < len(h.Ins)-p &&
		h.Del[len(h.Del)-1-s] == h.Ins[len(h.Ins)-1-s] {
		s++
	}

	return Hunk{
		Pos: h.Pos + uint64(p),
		Del: h.Del[p : len(h.Del)-s],
		Ins: h.Ins[p : len(h.Ins)-s],
	}
}

// splitLines splits text into lines KEEPING the trailing newline on each
// line, so concatenating the lines rebuilds the text exactly
func splitLines(text []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, text[start:i+1])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}
