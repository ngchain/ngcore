package ngp2p

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/ngchain/ngcore/utils"
)

// Wired type
type wiredProtocol struct {
	node *LocalNode // local host
}

func registerWired(node *LocalNode) *wiredProtocol {
	w := &wiredProtocol{
		node: node,
	}

	w.node.SetStreamHandler(WiredProtocol, func(stream network.Stream) {
		log.Debugf("handling new stream from %s", stream.Conn().RemotePeer())
		go w.handleStream(stream)
	})

	return w
}

func (w *wiredProtocol) handleStream(stream network.Stream) {
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
	var msg = &Message{}

	err = utils.Proto.Unmarshal(buf, msg)
	if err != nil {
		log.Error(err)
		return
	}

	if !verifyMessage(stream.Conn().RemotePeer(), msg) {
		w.reject(msg.Header.MessageId, stream, fmt.Errorf("message is invalid"))
		return
	}

	switch msg.Header.MessageType {
	case MessageType_PING:
		w.onPing(stream, msg)
	case MessageType_GETCHAIN:
		w.onGetChain(stream, msg)
	}

	_ = stream.Close()
}
