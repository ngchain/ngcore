package broadcast

import (
	"bytes"
	"math/big"
	"testing"
	"time"

	"github.com/c0mm4nd/rlp"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

// mineTestBlock seals a valid ZERONET block at height 1 on the genesis,
// so it passes the stateless CheckError gate of the block validator
func mineTestBlock(t *testing.T) *ngtypes.FullBlock {
	t.Helper()

	network := ngtypes.ZERONET
	parent := ngtypes.GetGenesisBlock(network)
	height := parent.GetHeight() + 1
	blockTime := ngtypes.GetGenesisTimestamp(network) + height*16

	diff := ngtypes.GetNextDiff(height, blockTime, parent)
	block := ngtypes.NewBareBlock(network, height, blockTime, parent.GetHash(), diff)

	miner, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	genTx := ngtypes.NewTx(network, ngtypes.GenerateTx, height,
		ngtypes.NewAddress(miner),
		ngtypes.GetBlockReward(height),
		big.NewInt(0), nil, nil)
	if err := genTx.Signature(miner); err != nil {
		t.Fatal(err)
	}

	if err := block.ToUnsealing([]*ngtypes.FullTx{genTx}); err != nil {
		t.Fatal(err)
	}

	for n := uint64(0); n < 1_000_000; n++ {
		if err := block.ToSealed(utils.PackUint64LE(n)); err != nil {
			t.Fatal(err)
		}
		if block.CheckError() == nil {
			return block
		}
	}

	t.Fatal("failed to seal a ZERONET block within 1e6 nonces")
	return nil
}

// signedTestTx returns a full-envelope signed tx which verifies
// statelessly, plus its signer
func signedTestTx(t *testing.T) (*ngtypes.FullTx, *ngtypes.PrivateKey) {
	t.Helper()

	key, err := ngtypes.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	tx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, 1,
		ngtypes.NewAddress(key),
		big.NewInt(42), big.NewInt(0), nil, nil)
	if err := tx.Signature(key); err != nil {
		t.Fatal(err)
	}

	return tx, key
}

func newBroadcast(t *testing.T, mn mocknet.Mocknet) *Broadcast {
	t.Helper()

	h, err := mn.GenPeer()
	if err != nil {
		t.Fatal(err)
	}

	b := NewBroadcastProtocol(h, ngtypes.ZERONET,
		make(chan *ngtypes.FullBlock, 8), make(chan *ngtypes.FullTx, 8))
	t.Cleanup(b.Close)

	return b
}

// newBroadcastPair builds two connected broadcasters on a mocknet and
// waits until the pubsub meshes see each other
func newBroadcastPair(t *testing.T) (*Broadcast, *Broadcast) {
	t.Helper()

	mn := mocknet.New()
	t.Cleanup(func() { _ = mn.Close() })

	b1 := newBroadcast(t, mn)
	b2 := newBroadcast(t, mn)

	if err := mn.LinkAll(); err != nil {
		t.Fatal(err)
	}
	if err := mn.ConnectAllButSelf(); err != nil {
		t.Fatal(err)
	}

	b1.GoServe()
	b2.GoServe()

	deadline := time.Now().Add(10 * time.Second)
	for {
		if len(b1.PubSub.ListPeers(b1.blockTopic)) > 0 &&
			len(b1.PubSub.ListPeers(b1.txTopic)) > 0 &&
			len(b2.PubSub.ListPeers(b2.blockTopic)) > 0 &&
			len(b2.PubSub.ListPeers(b2.txTopic)) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("pubsub meshes never converged")
		}
		time.Sleep(20 * time.Millisecond)
	}

	return b1, b2
}

func pubsubMsg(data []byte) *pubsub.Message {
	return &pubsub.Message{Message: &pubsub_pb.Message{Data: data}}
}

func mustEncode(t *testing.T, v interface{}) []byte {
	t.Helper()

	raw, err := rlp.EncodeToBytes(v)
	if err != nil {
		t.Fatal(err)
	}

	return raw
}

// blockCopy round-trips a block through rlp so tests can tamper a copy
func blockCopy(t *testing.T, block *ngtypes.FullBlock) *ngtypes.FullBlock {
	t.Helper()

	var cp ngtypes.FullBlock
	if err := rlp.DecodeBytes(mustEncode(t, block), &cp); err != nil {
		t.Fatal(err)
	}

	return &cp
}

// ---------------------------------------------------------------------------
// pub/sub delivery over the mocknet
// ---------------------------------------------------------------------------

func TestBroadcastBlockDelivery(t *testing.T) {
	b1, b2 := newBroadcastPair(t)

	block := mineTestBlock(t)
	if err := b1.BroadcastBlock(block); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-b2.OnBlock:
		if !bytes.Equal(got.GetHash(), block.GetHash()) {
			t.Errorf("received block %x, want %x", got.GetHash(), block.GetHash())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for the block broadcast")
	}
}

func TestBroadcastTxDelivery(t *testing.T) {
	b1, b2 := newBroadcastPair(t)

	tx, _ := signedTestTx(t)
	if err := b1.BroadcastTx(tx); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-b2.OnTx:
		if !bytes.Equal(got.GetHash(), tx.GetHash()) {
			t.Errorf("received tx %x, want %x", got.GetHash(), tx.GetHash())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for the tx broadcast")
	}
}

func TestBroadcastAfterClose(t *testing.T) {
	mn := mocknet.New()
	t.Cleanup(func() { _ = mn.Close() })

	b := newBroadcast(t, mn)
	b.GoServe()
	b.Close()

	if err := b.BroadcastBlock(mineTestBlock(t)); err == nil {
		t.Error("BroadcastBlock must fail on a closed topic")
	}

	tx, _ := signedTestTx(t)
	if err := b.BroadcastTx(tx); err == nil {
		t.Error("BroadcastTx must fail on a closed topic")
	}
}

func TestBroadcastEncodeErrors(t *testing.T) {
	mn := mocknet.New()
	t.Cleanup(func() { _ = mn.Close() })

	b := newBroadcast(t, mn)

	// negative big.Ints are not rlp-encodable
	badTx := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, 1,
		ngtypes.Address{}, big.NewInt(-1), big.NewInt(0), nil, nil)
	if err := b.BroadcastTx(badTx); err == nil {
		t.Error("BroadcastTx must fail on un-encodable tx")
	}

	badBlock := mineTestBlock(t)
	badBlock.Txs[0].Value = big.NewInt(-1)
	if err := b.BroadcastBlock(badBlock); err == nil {
		t.Error("BroadcastBlock must fail on un-encodable block")
	}
}

// ---------------------------------------------------------------------------
// message handlers
// ---------------------------------------------------------------------------

func TestOnBroadcastGarbage(t *testing.T) {
	mn := mocknet.New()
	t.Cleanup(func() { _ = mn.Close() })

	b := newBroadcast(t, mn)

	// garbage must be dropped without reaching the channels
	b.onBroadcastBlock(pubsubMsg([]byte{0xff}))
	b.onBroadcastTx(pubsubMsg([]byte{0xff}))

	select {
	case block := <-b.OnBlock:
		t.Errorf("unexpected block from garbage: %v", block)
	case tx := <-b.OnTx:
		t.Errorf("unexpected tx from garbage: %v", tx)
	case <-time.After(100 * time.Millisecond):
	}
}

// ---------------------------------------------------------------------------
// validators
// ---------------------------------------------------------------------------

func TestValidateBlockMsg(t *testing.T) {
	mn := mocknet.New()
	t.Cleanup(func() { _ = mn.Close() })

	b := newBroadcast(t, mn)
	valid := mineTestBlock(t)

	t.Run("valid block", func(t *testing.T) {
		if !b.validateBlockMsg(b.ctx, b.node.ID(), pubsubMsg(mustEncode(t, valid))) {
			t.Error("a valid block must pass")
		}
	})

	t.Run("oversize", func(t *testing.T) {
		if b.validateBlockMsg(b.ctx, b.node.ID(), pubsubMsg(make([]byte, ngtypes.MaxBlockBytes+1))) {
			t.Error("an oversize message must fail")
		}
	})

	t.Run("garbage", func(t *testing.T) {
		if b.validateBlockMsg(b.ctx, b.node.ID(), pubsubMsg([]byte{0xff})) {
			t.Error("garbage must fail")
		}
	})

	t.Run("wrong network", func(t *testing.T) {
		alien := blockCopy(t, valid)
		alien.BlockHeader.Network = ngtypes.MAINNET
		if b.validateBlockMsg(b.ctx, b.node.ID(), pubsubMsg(mustEncode(t, alien))) {
			t.Error("a block from another network must fail")
		}
	})

	t.Run("broken block", func(t *testing.T) {
		broken := blockCopy(t, valid)
		broken.BlockHeader.TxTrieHash[0] ^= 1
		if b.validateBlockMsg(b.ctx, b.node.ID(), pubsubMsg(mustEncode(t, broken))) {
			t.Error("a block failing CheckError must fail")
		}
	})
}

func TestValidateTxMsg(t *testing.T) {
	mn := mocknet.New()
	t.Cleanup(func() { _ = mn.Close() })

	b := newBroadcast(t, mn)
	valid, key := signedTestTx(t)

	t.Run("valid full envelope", func(t *testing.T) {
		if !b.validateTxMsg(b.ctx, b.node.ID(), pubsubMsg(mustEncode(t, valid))) {
			t.Error("a valid signed tx must pass")
		}
	})

	t.Run("oversize", func(t *testing.T) {
		if b.validateTxMsg(b.ctx, b.node.ID(), pubsubMsg(make([]byte, maxTxWireSize+1))) {
			t.Error("an oversize message must fail")
		}
	})

	t.Run("garbage", func(t *testing.T) {
		if b.validateTxMsg(b.ctx, b.node.ID(), pubsubMsg([]byte{0xff})) {
			t.Error("garbage must fail")
		}
	})

	t.Run("unsigned", func(t *testing.T) {
		unsigned := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, 1,
			ngtypes.NewAddress(key), big.NewInt(42), big.NewInt(0), nil, nil)
		if b.validateTxMsg(b.ctx, b.node.ID(), pubsubMsg(mustEncode(t, unsigned))) {
			t.Error("an unsigned tx must fail")
		}
	})

	t.Run("wrong network", func(t *testing.T) {
		alien := ngtypes.NewTx(ngtypes.MAINNET, ngtypes.GenerateTx, 1,
			ngtypes.NewAddress(key), big.NewInt(42), big.NewInt(0), nil, nil)
		if err := alien.Signature(key); err != nil {
			t.Fatal(err)
		}
		if b.validateTxMsg(b.ctx, b.node.ID(), pubsubMsg(mustEncode(t, alien))) {
			t.Error("a tx from another network must fail")
		}
	})

	// the default secp256k1 scheme uses the recover envelope, where a
	// tampered tx just recovers as SOMEONE ELSE's (eth-style); the
	// compact and tamper cases need a non-recovery scheme
	mldsaKey, err := ngtypes.GenerateSchemeKey(ngtypes.SchemeMLDSA44)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("compact envelope passes through", func(t *testing.T) {
		compact := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, 1,
			ngtypes.NewAddress(mldsaKey), big.NewInt(42), big.NewInt(0), nil, nil)
		if err := compact.SignatureCompact(mldsaKey); err != nil {
			t.Fatal(err)
		}
		if !compact.IsCompactEnvelope() {
			t.Fatal("expected a compact envelope")
		}
		if !b.validateTxMsg(b.ctx, b.node.ID(), pubsubMsg(mustEncode(t, compact))) {
			t.Error("a compact-envelope tx must pass through for the pool to judge")
		}
	})

	t.Run("tampered after signing", func(t *testing.T) {
		tampered := ngtypes.NewTx(ngtypes.ZERONET, ngtypes.GenerateTx, 1,
			ngtypes.NewAddress(mldsaKey), big.NewInt(42), big.NewInt(0), nil, nil)
		if err := tampered.Signature(mldsaKey); err != nil {
			t.Fatal(err)
		}
		tampered.Value = big.NewInt(43)
		if b.validateTxMsg(b.ctx, b.node.ID(), pubsubMsg(mustEncode(t, tampered))) {
			t.Error("a tampered tx must fail verification")
		}
	})
}
