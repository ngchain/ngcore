package ngp2p

import (
	"io/ioutil"

	"github.com/libp2p/go-libp2p-core/network"
	"google.golang.org/protobuf/proto"
)

// pattern: /ngp2p/protocol-name/version
const (
	protocolVersion = "0.0.2"
	channal         = "/ngp2p/wired/" + protocolVersion
)

// Wired type
type wiredProtocol struct {
	node *LocalNode // local host
}

func registerWired(node *LocalNode) *wiredProtocol {
	w := &wiredProtocol{
		node: node,
	}

	w.node.SetStreamHandler(channal, func(stream network.Stream) {
		buf, err := ioutil.ReadAll(stream)
		if err != nil {
			log.Error(err)
			return
		}

		// unmarshal it
		var msg = &Message{}

		err = proto.Unmarshal(buf, msg)
		if err != nil {
			log.Error(err)
			return
		}

		if !verifyMessage(stream.Conn().RemotePeer(), msg) {
			log.Errorf("failed to authenticate message")
			return
		}

		switch msg.Header.MessageType {
		case MessageType_PING:
			w.onPing(stream, msg)
		case MessageType_CHAIN:
			w.onChain(stream, msg)
		}

		_ = stream.Close()
	})

	return w
}
