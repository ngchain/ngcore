package ngp2p

import (
	"context"
	"fmt"
	"github.com/gogo/protobuf/io"
	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/helpers"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	multiplex "github.com/libp2p/go-libp2p-mplex"
	yamux "github.com/libp2p/go-libp2p-yamux"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	"github.com/libp2p/go-tcp-transport"
	"github.com/ngchain/ngcore/ngchain"
	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/ngsheet"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/txpool"
	"go.uber.org/atomic"
	"sync"
	"time"
)

type LocalNode struct {
	host.Host // lib-p2p host
	*Wired
	*Broadcaster

	isInitialized *atomic.Bool

	isSyncedCh  chan bool
	OnSynced    func()
	OnNotSynced func()

	sheetManager *ngsheet.Manager
	Chain        *ngchain.Chain
	TxPool       *txpool.TxPool

	RemoteHeights   *sync.Map // key:id value:height
	isStrictMode    bool
	isBootstrapNode bool
}

// Create a new node with its implemented protocols
func NewLocalNode(port int, isStrictMode, isBootstrapNode bool, sheetManager *ngsheet.Manager, chain *ngchain.Chain, txPool *txpool.TxPool) *LocalNode {
	ctx := context.Background()

	priv := getP2PKey(true) //isBootstrap)

	transports := libp2p.ChainOptions(
		libp2p.Transport(tcp.NewTCPTransport),
		//libp2p.Transport(ws.New),
	)

	listenAddrs := libp2p.ListenAddrStrings(
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port),
		fmt.Sprintf("/ip6/::/tcp/%d", port),
	)

	muxers := libp2p.ChainOptions(
		libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport),
		libp2p.Muxer("/mplex/6.7.0", multiplex.DefaultTransport),
	)

	var p2pDHT *dht.IpfsDHT
	newDHT := func(h host.Host) (routing.PeerRouting, error) {
		var err error
		p2pDHT, err = dht.New(ctx, h)
		return p2pDHT, err
	}

	localHost, err := libp2p.New(
		ctx,
		transports,
		listenAddrs,
		muxers,
		libp2p.Identity(priv),
		libp2p.Routing(newDHT),
	)
	if err != nil {
		panic(err)
	}

	// init
	for _, addr := range localHost.Addrs() {
		log.Infof("Listening P2P on %s/p2p/%s", addr.String(), localHost.ID().String())
	}

	mdns, err := discovery.NewMdnsService(ctx, localHost, time.Second*10, "") // using ipfs network
	if err != nil {
		panic(err)
	}
	peerInfoCh := make(chan peer.AddrInfo)
	mdns.RegisterNotifee(
		&mdnsNotifee{
			h:          localHost,
			ctx:        ctx,
			PeerInfoCh: peerInfoCh,
		},
	)

	node := &LocalNode{
		Host:        localHost,
		Wired:       nil,
		Broadcaster: nil,

		isInitialized:   atomic.NewBool(false),
		isBootstrapNode: isBootstrapNode,
		isSyncedCh:      make(chan bool),
		OnSynced:        nil,
		OnNotSynced:     nil,

		sheetManager:  sheetManager,
		Chain:         chain,
		TxPool:        txPool,
		RemoteHeights: new(sync.Map),
		isStrictMode:  isStrictMode,
	}

	node.Broadcaster = registerBroadcaster(node)
	node.Wired = registerProtocol(node)

	go node.Wired.Sync()

	// mdns seeding
	go func() {
		for {
			pi := <-peerInfoCh // will block untill we discover a peer
			log.Infof("Found peer:", pi, ", connecting")
			if err := node.Connect(ctx, pi); err != nil {
				log.Errorf("Connection failed: %s", err)
				continue
			}
			node.Ping(pi.ID)
		}
	}()

	err = p2pDHT.Bootstrap(ctx)
	if err != nil {
		panic(err)
	}

	return node
}

// Authenticate incoming p2p message
// message: a protobufs go data object
// data: common p2p message data
func (n *LocalNode) verifyResponse(message *pb.Message) bool {
	if _, exists := n.requests.Load(message.Header.Uuid); !exists {
		// remove request from map as we have processed it here
		return false
	}

	n.requests.Delete(message.Header.Uuid)
	return true
}

func (n *LocalNode) authenticateMessage(message *pb.Message) bool {
	sign := message.Header.Sign
	message.Header.Sign = nil

	raw, err := proto.Marshal(message)
	if err != nil {
		log.Errorf("failed to marshal pb message: %v", err)
		return false
	}

	message.Header.Sign = sign
	peerId, err := peer.Decode(message.Header.Uuid)
	if err != nil {
		log.Errorf("Failed to decode node id from base58: %v", err)
		return false
	}

	return n.verifyData(raw, sign, peerId, message.Header.PeerKey)
}

// sign an outgoing p2p message payload
func (n *LocalNode) signMessage(message *pb.Message) ([]byte, error) {
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

// NewHeader is a helper method: generate message data shared between all node's p2p protocols
// messageId: unique for requests, copied from request for responses
func (n *LocalNode) NewHeader(uuid string) *pb.Header {
	// Add protobufs bin data for message author public key
	// this is useful for authenticating  messages forwarded by a node authored by another node
	peerKey, err := n.Peerstore().PubKey(n.ID()).Bytes()

	if err != nil {
		panic("Failed to get public key for sender from local peer store.")
	}

	return &pb.Header{
		NetworkId: ngtypes.NetworkId,
		Uuid:      uuid,
		Timestamp: 0,
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
		_ = s.Reset()
		return false
	}

	// FullClose closes the stream and waits for the other side to close their half.
	err = helpers.FullClose(s)
	if err != nil {
		log.Error(err)
		_ = s.Reset()
		return false
	}

	return true
}
