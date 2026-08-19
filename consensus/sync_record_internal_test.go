package consensus

import (
	"bytes"
	"math/big"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/ngchain/ngcore/ngtypes"
)

func TestNewRemoteRecord(t *testing.T) {
	tests := []struct {
		name             string
		latest           uint64
		wantCheckpointAt uint64
	}{
		{"latest on a checkpoint", 2 * ngtypes.BlockCheckRound, 2 * ngtypes.BlockCheckRound},
		{"latest between checkpoints", 2*ngtypes.BlockCheckRound + 5, 2 * ngtypes.BlockCheckRound},
		{"latest below the first checkpoint", ngtypes.BlockCheckRound - 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRemoteRecord("peer", 0, tt.latest, []byte{0x01}, []byte{0x02})
			if r.checkpointHeight != tt.wantCheckpointAt {
				t.Fatalf("checkpointHeight = %d, want %d", r.checkpointHeight, tt.wantCheckpointAt)
			}
			if r.latest != tt.latest {
				t.Fatalf("latest = %d, want %d", r.latest, tt.latest)
			}
			if r.checkpointActualDiff.Cmp(big.NewInt(0x02)) != 0 {
				t.Fatalf("checkpointActualDiff = %s, want 2", r.checkpointActualDiff)
			}
		})
	}
}

func TestRemoteRecordUpdate(t *testing.T) {
	r := NewRemoteRecord("peer", 0, 10, []byte{0x01}, []byte{0x02})

	r.update(5, 37, []byte{0x0a}, []byte{0x0b})

	if r.origin != 5 || r.latest != 37 {
		t.Fatalf("update kept origin=%d latest=%d, want 5/37", r.origin, r.latest)
	}
	if r.checkpointHeight != 30 {
		t.Fatalf("checkpointHeight = %d, want 30", r.checkpointHeight)
	}
	if !bytes.Equal(r.checkpointHash, []byte{0x0a}) {
		t.Fatalf("checkpointHash = %x, want 0a", r.checkpointHash)
	}
	if r.checkpointActualDiff.Cmp(big.NewInt(0x0b)) != 0 {
		t.Fatalf("checkpointActualDiff = %s, want 11", r.checkpointActualDiff)
	}
}

func TestShouldSync(t *testing.T) {
	round := uint64(ngtypes.BlockCheckRound)

	tests := []struct {
		name         string
		remoteLatest uint64
		localHeight  uint64
		failedAt     int64
		want         bool
	}{
		{"remote a full round ahead", 3 * round, round, 0, true},
		{"remote ahead within the round", round + 5, round, 0, false},
		{"remote behind", round, 3 * round, 0, false},
		{"equal heights", round, round, 0, false},
		{"recently failed remote is ignored", 3 * round, round, time.Now().Unix(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRemoteRecord("peer", 0, tt.remoteLatest, nil, nil)
			r.lastFailedTime = tt.failedAt
			if got := r.shouldSync(tt.localHeight); got != tt.want {
				t.Fatalf("shouldSync(%d) = %v, want %v", tt.localHeight, got, tt.want)
			}
		})
	}
}

func TestShouldConverge(t *testing.T) {
	cp := ngtypes.GetGenesisBlock(ngtypes.ZERONET) // the local latest checkpoint
	cpHash := cp.GetHash()
	otherHash := bytes.Repeat([]byte{0xee}, 32)
	heavier := new(big.Int).Add(cp.GetActualDiff(), big.NewInt(1)).Bytes()
	lighter := []byte{0x01} // diff 1, lighter than any real pow

	round := uint64(ngtypes.BlockCheckRound)

	tests := []struct {
		name         string
		remoteHash   []byte
		remoteLatest uint64
		remoteDiff   []byte
		localHeight  uint64
		failedAt     int64
		want         bool
	}{
		{"same checkpoint", cpHash, 2 * round, heavier, 5, 0, false},
		{"higher checkpoint level", otherHash, 2 * round, lighter, 5, 0, true},
		{"same level but heavier checkpoint", otherHash, 8, heavier, 5, 0, true},
		{"same level and lighter checkpoint", otherHash, 8, lighter, 5, 0, false},
		{"remote not ahead", otherHash, 5, heavier, 5, 0, false},
		{"recently failed remote is ignored", otherHash, 2 * round, heavier, 5, time.Now().Unix(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRemoteRecord("peer", 0, tt.remoteLatest, tt.remoteHash, tt.remoteDiff)
			r.lastFailedTime = tt.failedAt
			if got := r.shouldConverge(cp, tt.localHeight); got != tt.want {
				t.Fatalf("shouldConverge(local@%d) = %v, want %v", tt.localHeight, got, tt.want)
			}
		})
	}
}

func TestRecordFailure(t *testing.T) {
	r := NewRemoteRecord("peer", 0, 100, nil, nil)

	// the first 3 failures are forgiven
	for i := 0; i < 3; i++ {
		r.recordFailure()
	}
	if r.lastFailedTime != 0 {
		t.Fatal("3 failures must not blocklist the remote yet")
	}

	// the 4th failure starts the cooldown
	r.recordFailure()
	if r.lastFailedTime == 0 {
		t.Fatal("4 failures must blocklist the remote")
	}
	if r.shouldSync(0) {
		t.Fatal("a blocklisted remote must not be synced from")
	}
}

func TestPutRemote(t *testing.T) {
	pow := newBarePow(t, ngtypes.ZERONET)
	mod := newSyncModule(pow, nil)

	r := NewRemoteRecord("peer", 0, 42, nil, nil)
	mod.putRemote(peer.ID("peer"), r)

	if got := mod.store[peer.ID("peer")]; got != r {
		t.Fatal("putRemote did not store the record")
	}
}

func TestMustSync(t *testing.T) {
	pow := newBarePow(t, ngtypes.ZERONET)
	mod := newSyncModule(pow, nil)

	ahead := NewRemoteRecord("ahead", 0, 3*ngtypes.BlockCheckRound, nil, nil)
	behind := NewRemoteRecord("behind", 0, 1, nil, nil)

	got := mod.MustSync([]*RemoteRecord{ahead, behind})
	if len(got) != 1 || got[0] != ahead {
		t.Fatalf("MustSync selected %d records, want just the remote ahead", len(got))
	}

	if got := mod.MustSync(nil); len(got) != 0 {
		t.Fatal("MustSync without remotes must select nothing")
	}
}

func TestMustConverge(t *testing.T) {
	pow := newBarePow(t, ngtypes.ZERONET)
	mod := newSyncModule(pow, nil)

	otherHash := bytes.Repeat([]byte{0xee}, 32)
	forked := NewRemoteRecord("forked", 0, 3*ngtypes.BlockCheckRound, otherHash, []byte{0x01})
	same := NewRemoteRecord("same", 0, 3*ngtypes.BlockCheckRound,
		pow.Chain.GetLatestCheckpoint().GetHash(), []byte{0x01})

	got := mod.MustConverge([]*RemoteRecord{forked, same})
	if len(got) != 1 || got[0] != forked {
		t.Fatalf("MustConverge selected %d records, want just the forked remote", len(got))
	}

	// on public networks converging demands enough independent remotes
	publicPow := newBarePow(t, ngtypes.TESTNET)
	publicMod := newSyncModule(publicPow, nil)
	if got := publicMod.MustConverge([]*RemoteRecord{forked}); got != nil {
		t.Fatal("a public network must suppress converging with too few peers")
	}
}
