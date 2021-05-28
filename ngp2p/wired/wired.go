package wired

import (
	"fmt"
	"github.com/ngchain/ngcore/ngtypes/ngproto"

	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/ngchain/ngcore/ngchain"

	logging "github.com/ipfs/go-log/v2"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-msgio"

	"github.com/ngchain/ngcore/ngp2p/defaults"
	"github.com/ngchain/ngcore/ngp2p/message"

	"github.com/libp2p/go-libp2p-core/network"

	"github.com/ngchain/ngcore/utils"
)

var log = logging.Logger("wired")

// Wired type
type Wired struct {
	network ngproto.NetworkType
	host    core.Host // local host

	protocolID protocol.ID

	chain *ngchain.Chain
}

func NewWiredProtocol(host core.Host, network ngproto.NetworkType, chain *ngchain.Chain) *Wired {
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
	var msg = &message.Message{}

	err = utils.Proto.Unmarshal(raw, msg)
	if err != nil {
		log.Error(err)
		return
	}

	if !Verify(stream.Conn().RemotePeer(), msg) {
		w.sendReject(msg.Header.MessageId, stream, fmt.Errorf("message is invalid"))
		return
	}

	switch msg.Header.MessageType {
	case message.MessageType_PING:
		w.onPing(stream, msg)
	case message.MessageType_GETCHAIN:
		w.onGetChain(stream, msg)
	case message.MessageType_GETSHEET:
		w.onGetChain(stream, msg)
	default:
		w.sendReject(msg.Header.MessageId, stream, fmt.Errorf("unsupported protocol method"))
	}

	err = stream.Close()
	if err != nil {
		log.Error(err)
	}
}
