package ngtypes

// some pre-defined extra formats for the smart contracts

import (
	"bytes"

	"github.com/pkg/errors"
)

// Hunk is one replacement in the contract text: at Pos (a byte offset in
// the ORIGINAL text) the bytes Del are removed and Ins are inserted.
// A pure insertion has an empty Del; a pure deletion has an empty Ins
type Hunk struct {
	Pos uint64
	Del []byte
	Ins []byte
}

// EditExtra is the Extra payload of an Edit Tx: a whole patch applied
// atomically. Hunks use original-text coordinates, must be sorted
// ascending and must not overlap
type EditExtra struct {
	Hunks []Hunk
}

var (
	ErrHunkNone       = errors.New("edit contains no effective hunk")
	ErrHunkOutOfBound = errors.New("hunk is out of the text bound")
	ErrHunkOverlap    = errors.New("hunks overlap or are not sorted")
	ErrHunkMismatch   = errors.New("hunk Del does not match the original text")
)

// ApplyEdits applies the patch onto text, returning the new text.
// The input text is never mutated; any invalid hunk fails the whole patch
func ApplyEdits(text []byte, hunks []Hunk) ([]byte, error) {
	if len(hunks) == 0 {
		return nil, ErrHunkNone
	}

	out := make([]byte, 0, len(text))
	cursor := uint64(0)

	for _, h := range hunks {
		if len(h.Del) == 0 && len(h.Ins) == 0 {
			return nil, ErrHunkNone
		}
		if h.Pos < cursor {
			return nil, ErrHunkOverlap
		}
		end := h.Pos + uint64(len(h.Del))
		if h.Pos > uint64(len(text)) || end > uint64(len(text)) {
			return nil, ErrHunkOutOfBound
		}
		if !bytes.Equal(text[h.Pos:end], h.Del) {
			return nil, ErrHunkMismatch
		}

		out = append(out, text[cursor:h.Pos]...)
		out = append(out, h.Ins...)
		cursor = end
	}

	return append(out, text[cursor:]...), nil
}

// DiffHunks computes a minimal line-based patch turning oldText into
// newText, suitable for ApplyEdits. It is a plain LCS diff, so
// ApplyEdits(oldText, DiffHunks(oldText, newText)) == newText always
// holds; identical texts yield no hunks
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
		// common line: close nothing, advance both
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
