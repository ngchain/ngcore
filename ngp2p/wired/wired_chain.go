package wired

import (
	"github.com/libp2p/go-libp2p-core/network"

	"github.com/ngchain/ngcore/ngp2p/message"

	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

// sendChain will send peer the specific vault's sendChain, which's len is not must be full BlockCheckRound num
func (w *Wired) sendChain(uuid []byte, stream network.Stream, blocks ...*ngtypes.Block) bool {
	if len(blocks) == 0 {
		return false
	}

	log.Debugf("replying sendChain to %s. Message id: %x, from block@%d to %d",
		stream.Conn().RemotePeer(), uuid, blocks[0].GetHeight(), blocks[len(blocks)-1].GetHeight(),
	)

	payload, err := utils.Proto.Marshal(&message.ChainPayload{
		Blocks: blocks,
	})
	if err != nil {
		log.Debugf("failed to sign pb data: %s", err)
		return false
	}

	// create message data
	resp := &message.Message{
		Header:  NewHeader(w.host, w.network, uuid, message.MessageType_CHAIN),
		Payload: payload,
	}

	// sign the data
	signature, err := Signature(w.host, resp)
	if err != nil {
		log.Debugf("failed to sign pb data")
		return false
	}

	// add the signature to the message
	resp.Header.Sign = signature

	err = Reply(stream, resp)
	if err != nil {
		log.Debugf("sendChain to: %s was sent. Message Id: %x", stream.Conn().RemotePeer(), resp.Header.MessageId)
		return false
	}

	log.Debugf("sendChain to: %s was sent. Message Id: %x", stream.Conn().RemotePeer(), resp.Header.MessageId)

	return true
}

// DecodeChainPayload unmarshal the raw and return the *pb.ChainPayload.
func DecodeChainPayload(rawPayload []byte) (*message.ChainPayload, error) {
	payload := &message.ChainPayload{}

	err := utils.Proto.Unmarshal(rawPayload, payload)
	if err != nil {
		return nil, err
	}

	return payload, nil
}
