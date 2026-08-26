package wired

import (
	"github.com/c0mm4nd/rlp"
	core "github.com/libp2p/go-libp2p/core"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-msgio"
	logging "github.com/ngchain/zap-log"

	"github.com/ngchain/ngcore/blockchain"
	"github.com/ngchain/ngcore/ngp2p/defaults"
	"github.com/ngchain/ngcore/ngtypes"
)

var log = logging.Logger("wired")

// Wired type.
type Wired struct {
	network ngtypes.Network
	host    core.Host // local host

	protocolID protocol.ID

	chain *blockchain.Chain

	// OnStemTx / OnStemCommit, when set, receive validated Dandelion++
	// stem-phase items together with the remaining TTL. They keep ngp2p
	// decoupled from the pool: the wired layer never touches the mempool,
	// the registered router decides to stem onward or to fluff.
	OnStemTx     func(tx *ngtypes.FullTx, ttl uint8)
	OnStemCommit func(commit *ngtypes.Commitment, ttl uint8)
}

func NewWiredProtocol(host core.Host, network ngtypes.Network, chain *blockchain.Chain) *Wired {
	w := &Wired{
		network: network,
		host:    host,

		protocolID: protocol.ID(defaults.GetWiredProtocol(network)),

		chain: chain,
	}

	return w
}

func (w *Wired) GetWiredProtocol() protocol.ID {
	return w.protocolID
}

func (w *Wired) GoServe() {
	// register handler
	w.host.SetStreamHandler(w.protocolID, func(stream network.Stream) {
		log.Debugf("handling new stream from %s", stream.Conn().RemotePeer())
		go w.handleStream(stream)
	})
}

func (w *Wired) handleStream(stream network.Stream) {
	r := msgio.NewReader(stream)
	raw, err := r.ReadMsg()
	if err != nil {
		log.Error(err)
		return
	}

	// unmarshal it
	var msg Message

	err = rlp.DecodeBytes(raw, &msg)
	if err != nil {
		log.Error(err)
		return
	}

	if !Verify(stream.Conn().RemotePeer(), &msg) {
		w.sendReject(msg.Header.ID, stream, ErrMsgSignInvalid)
		return
	}

	switch msg.Header.Type {
	case PingMsg:
		w.onPing(stream, &msg)
	case GetChainMsg:
		w.onGetChain(stream, &msg)
	case GetSheetMsg:
		w.onGetSheet(stream, &msg)
	case StemTxMsg:
		w.onStemTx(stream, &msg)
	case StemCommitMsg:
		w.onStemCommit(stream, &msg)
	default:
		w.sendReject(msg.Header.ID, stream, ErrMsgTypeInvalid)
	}

	err = stream.Close()
	if err != nil {
		log.Error(err)
	}
}
