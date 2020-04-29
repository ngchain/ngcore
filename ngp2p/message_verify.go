package ngp2p

import (
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ngchain/ngcore/ngp2p/pb"
	"github.com/ngchain/ngcore/utils"
)

// Authenticate incoming p2p message.
func (n *LocalNode) verifyResponse(message *pb.Message) bool {
	if _, exists := n.requests.Load(message.Header.Uuid); !exists {
		return false
	}

	n.requests.Delete(message.Header.Uuid)

	return true
}

func (n *LocalNode) verifyMessage(remotePeerID peer.ID, message *pb.Message) bool {
	sign := message.Header.Sign
	message.Header.Sign = nil

	raw, err := utils.Proto.Marshal(message)
	if err != nil {
		log.Errorf("failed to marshal pb message: %v", err)
		return false
	}

	message.Header.Sign = sign

	return n.verifyData(raw, sign, remotePeerID, message.Header.PeerKey)
}

// sign an outgoing p2p message payload.
func (n *LocalNode) signMessage(message *pb.Message) ([]byte, error) {
	message.Header.Sign = nil

	data, err := utils.Proto.Marshal(message)
	if err != nil {
		return nil, err
	}

	return n.signData(data)
}

// sign binary data using the local node's private key.
func (n *LocalNode) signData(data []byte) ([]byte, error) {
	key := n.Peerstore().PrivKey(n.ID())
	res, err := key.Sign(data)

	return res, err
}

// verifyData verifies incoming p2p message data integrity.
func (n *LocalNode) verifyData(data []byte, signature []byte, peerID peer.ID, pubKeyData []byte) bool {
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
	if idFromKey != peerID {
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
