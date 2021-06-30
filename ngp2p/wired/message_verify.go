package wired

import (
	"github.com/c0mm4nd/rlp"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/ngchain/ngcore/ngp2p/message"
)

// Verify verifies the data and sign in message
func Verify(peerID peer.ID, message *message.Message) bool {
	sign := message.Header.Sign
	message.Header.Sign = nil

	raw, err := rlp.EncodeToBytes(message)
	if err != nil {
		log.Errorf("failed to marshal pb message: %v", err)
		return false
	}

	message.Header.Sign = sign

	return verifyMessageData(raw, sign, peerID, message.Header.PeerKey)
}

// Signature an outgoing p2p message payload.
func Signature(host core.Host, message *message.Message) ([]byte, error) {
	message.Header.Sign = nil

	data, err := rlp.EncodeToBytes(message)
	if err != nil {
		return nil, err
	}

	key := host.Peerstore().PrivKey(host.ID())
	res, err := key.Sign(data)

	return res, err
}

// verifyMessageData verifies incoming p2p message data integrity.
// it is included in Verify so plz using Verify.
func verifyMessageData(data, signature []byte, peerID peer.ID, pubKeyData []byte) bool {
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
