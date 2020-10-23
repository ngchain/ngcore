package wired

import (
	"fmt"

	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngtypes"

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
	network ngtypes.NetworkType
	host    core.Host // local host

	chain *ngchain.Chain
}

func NewWiredProtocol(host core.Host, network ngtypes.NetworkType, chain *ngchain.Chain) *Wired {
	w := &Wired{
		network: network,
		host:    host,
		chain:   chain,
	}

	return w
}

func (w *Wired) GoServe() {
	// register handler
	w.host.SetStreamHandler(defaults.WiredProtocol, func(stream network.Stream) {
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
	}

	err = stream.Close()
	if err != nil {
		log.Error(err)
	}
}
