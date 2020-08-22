package wired

import (
	"fmt"
	logging "github.com/ipfs/go-log/v2"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/ngchain/ngcore/ngp2p/defaults"
	"github.com/ngchain/ngcore/ngp2p/message"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngchain/ngcore/utils"
)

var log = logging.Logger("wired")

// Wired type
type Wired struct {
	host core.Host // local host
}

func NewWiredProtocol(host core.Host) *Wired {
	w := &Wired{
		host: host,
	}

	// register handler
	host.SetStreamHandler(defaults.WiredProtocol, func(stream network.Stream) {
		log.Debugf("handling new stream from %s", stream.Conn().RemotePeer())
		go w.handleStream(stream)
	})

	return w
}

func (w *Wired) handleStream(stream network.Stream) {
	buf := make([]byte, 20480) // 20m
	l, err := stream.Read(buf)
	if err != nil {
		log.Error(err)
		return
	}

	buf = buf[:l]

	if l == 0 {
		return
	}

	// unmarshal it
	var msg = &message.Message{}

	err = utils.Proto.Unmarshal(buf, msg)
	if err != nil {
		log.Error(err)
		return
	}

	if !message.Verify(stream.Conn().RemotePeer(), msg) {
		w.reject(msg.Header.MessageId, stream, fmt.Errorf("message is invalid"))
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
