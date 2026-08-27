package lightclient

import (
	"bytes"
	"testing"

	"github.com/ngchain/ngcore/ngp2p/wired"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/statetrie"
)

// buildProof builds an inclusion or absence proof over an in-memory trie for a
// single balance-domain leaf, returning the response + the header binding it.
func buildProof(t *testing.T, addr []byte, value []byte) (*ngtypes.BlockHeader, *wired.ProofResponse) {
	t.Helper()

	store := statetrie.NewMemStore()
	path := statetrie.LeafPath(statetrie.DomainBalance, addr)

	var valueHash []byte
	if len(value) == 0 {
		valueHash = statetrie.ZeroHash()
	} else {
		valueHash = statetrie.ValueHash(value)
	}
	if err := statetrie.Update(store, path, valueHash); err != nil {
		t.Fatal(err)
	}

	root := statetrie.Root(store)
	proof := statetrie.Prove(store, path)

	header := &ngtypes.BlockHeader{StateRoot: root}
	resp := &wired.ProofResponse{
		Height:    7,
		BlockHash: header.GetHash(),
		StateRoot: root,
		Domain:    "balance",
		Key:       addr,
		Value:     value,
		ValueHash: valueHash,
		Path:      path,
		Proof:     proof,
		Found:     len(value) != 0,
	}
	return header, resp
}

func TestVerifyProofInclusion(t *testing.T) {
	addr := bytes.Repeat([]byte{0x11}, 32)
	val := []byte{0x2a}
	header, resp := buildProof(t, addr, val)

	got, ok := VerifyProof(header, resp)
	if !ok {
		t.Fatal("a valid inclusion proof must verify")
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("value = %x, want %x", got, val)
	}
}

func TestVerifyProofAbsence(t *testing.T) {
	addr := bytes.Repeat([]byte{0x22}, 32)
	header, resp := buildProof(t, addr, nil)

	got, ok := VerifyProof(header, resp)
	if !ok {
		t.Fatal("a valid absence proof must verify")
	}
	if got != nil {
		t.Fatalf("absence must return nil value, got %x", got)
	}
}

func TestVerifyProofRejectsWrongRoot(t *testing.T) {
	addr := bytes.Repeat([]byte{0x33}, 32)
	_, resp := buildProof(t, addr, []byte{0x01})

	// header the proof was NOT bound to
	other := &ngtypes.BlockHeader{StateRoot: bytes.Repeat([]byte{0xff}, 32)}
	if _, ok := VerifyProof(other, resp); ok {
		t.Fatal("a proof must not verify against a different StateRoot")
	}
}

func TestVerifyProofRejectsWrongBlockHash(t *testing.T) {
	addr := bytes.Repeat([]byte{0x44}, 32)
	header, resp := buildProof(t, addr, []byte{0x01})

	resp.BlockHash = bytes.Repeat([]byte{0xab}, 32) // no longer the header's hash
	if _, ok := VerifyProof(header, resp); ok {
		t.Fatal("a proof whose BlockHash != header hash must be rejected")
	}
}

func TestVerifyProofRejectsTamperedValue(t *testing.T) {
	addr := bytes.Repeat([]byte{0x55}, 32)
	header, resp := buildProof(t, addr, []byte{0x01})

	resp.Value = []byte{0x02} // valueHash no longer matches
	if _, ok := VerifyProof(header, resp); ok {
		t.Fatal("a tampered value must be rejected")
	}
}

func TestVerifyProofRejectsTamperedBranch(t *testing.T) {
	addr := bytes.Repeat([]byte{0x66}, 32)
	header, resp := buildProof(t, addr, []byte{0x01})

	resp.Proof[0] = bytes.Repeat([]byte{0x99}, statetrie.HashSize)
	if _, ok := VerifyProof(header, resp); ok {
		t.Fatal("a tampered branch must be rejected")
	}
}

func TestVerifyProofRejectsFoundWithoutValue(t *testing.T) {
	addr := bytes.Repeat([]byte{0x77}, 32)
	header, resp := buildProof(t, addr, []byte{0x01})

	resp.Value = nil // Found=true but empty value is inconsistent
	if _, ok := VerifyProof(header, resp); ok {
		t.Fatal("Found with an empty value must be rejected")
	}
}

func TestVerifyProofNilArgs(t *testing.T) {
	if _, ok := VerifyProof(nil, nil); ok {
		t.Fatal("nil args must not verify")
	}
}
