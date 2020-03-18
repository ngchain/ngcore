package ngp2p

import (
	"context"
	"github.com/gogo/protobuf/io"
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/helpers"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/ngin-network/ngcore/chain"
	"github.com/ngin-network/ngcore/ngtypes"
	"github.com/ngin-network/ngcore/sheetManager"
	"github.com/ngin-network/ngcore/txpool"
	"sync"
)

type LocalNode struct {
	host.Host // lib-p2p host
	*Protocol
	sheetManager *sheetManager.SheetManager
	Chain        *chain.Chain
	txPool       *txpool.TxPool

	RemoteHeights *sync.Map // key:id value:height
}

// Create a new node with its implemented protocols
func NewLocalNode(host host.Host, done chan bool, sheetManager *sheetManager.SheetManager, chain *chain.Chain, txPool *txpool.TxPool) *LocalNode {
	node := &LocalNode{
		Host:         host,
		sheetManager: sheetManager,
		Chain:        chain,
		txPool:       txPool,

		RemoteHeights: new(sync.Map),
	}
	node.Protocol = RegisterProtocol(node, done)
	go node.Protocol.Sync()
	return node
}

// Authenticate incoming p2p message
// message: a protobufs go data object
// data: common p2p message data
func (n *LocalNode) authenticateMessage(message proto.Message, data *ngtypes.P2PHeader) bool {
	return true
}

// sign an outgoing p2p message payload
func (n *LocalNode) signProtoMessage(message proto.Message) ([]byte, error) {
	data, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	return n.signData(data)
}

// sign binary data using the local node's private key
func (n *LocalNode) signData(data []byte) ([]byte, error) {
	key := n.Peerstore().PrivKey(n.ID())
	res, err := key.Sign(data)
	return res, err
}

// Verify incoming p2p message data integrity
// data: data to verify
// signature: author signature provided in the message payload
// peerId: author peer id from the message payload
// pubKeyData: author public key from the message payload
func (n *LocalNode) verifyData(data []byte, signature []byte, peerId peer.ID, pubKeyData []byte) bool {
	key, err := crypto.UnmarshalPublicKey(pubKeyData)
	if err != nil {
		log.Error(err, "Failed to extract key from message key data")
		return false
	}

	// extract node id from the provided public key
	idFromKey, err := peer.IDFromPublicKey(key)

	if err != nil {
		log.Error(err, "Failed to extract peer id from public key")
		return false
	}

	// verify that message author node id matches the provided node public key
	if idFromKey != peerId {
		log.Error(err, "LocalNode id and provided public key mismatch")
		return false
	}

	res, err := key.Verify(data, signature)
	if err != nil {
		log.Error(err, "Error authenticating data")
		return false
	}

	return res
}

// helper method - generate message data shared between all node's p2p protocols
// messageId: unique for requests, copied from request for responses
func (n *LocalNode) NewP2PHeader(uuid string, broadcast bool) *ngtypes.P2PHeader {
	// Add protobufs bin data for message author public key
	// this is useful for authenticating  messages forwarded by a node authored by another node
	peerKey, err := n.Peerstore().PubKey(n.ID()).Bytes()

	if err != nil {
		panic("Failed to get public key for sender from local peer store.")
	}

	return &ngtypes.P2PHeader{
		NetworkId: ngtypes.NetworkId,
		Uuid:      uuid,
		Timestamp: 0,
		Broadcast: broadcast,
		PeerKey:   peerKey,
		Sign:      nil,
	}
}

// helper method - writes a protobuf go data object to a network stream
// data: reference of protobuf go data object to send (not the object itself)
// s: network stream to write the data to
func (n *LocalNode) sendProtoMessage(peerID peer.ID, method protocol.ID, data proto.Message) bool {
	s, err := n.NewStream(context.Background(), peerID, method)
	if err != nil {
		log.Error(err)
		return false
	}
	writer := io.NewFullWriter(s)
	err = writer.WriteMsg(data)
	if err != nil {
		log.Error(err)
		s.Reset()
		return false
	}
	// FullClose closes the stream and waits for the other side to close their half.
	err = helpers.FullClose(s)
	if err != nil {
		log.Error(err)
		s.Reset()
		return false
	}
	return true
}

func (n *LocalNode) IsSynced() bool {
	var total = 0
	var synced = 0
	localHeight := n.Chain.GetLatestBlockHeight()

	n.RemoteHeights.Range(func(_, value interface{}) bool {
		total++
		if localHeight+ngtypes.BlockCheckRound >= value.(uint64) {
			synced++
		}
		return true
	})

	return float64(synced)/float64(total) > 0.7
}
