package wired

import (
	"github.com/c0mm4nd/rlp"
	"github.com/libp2p/go-libp2p-core/network"

	"github.com/ngchain/ngcore/ngp2p/message"
	"github.com/ngchain/ngcore/ngtypes"
)

func (w *Wired) sendSheet(uuid []byte, stream network.Stream, sheet *ngtypes.Sheet) bool {
	log.Debugf("sending sheet to %s. Message id: %x...", stream.Conn().RemotePeer(), uuid)

	pongPayload := &message.SheetPayload{
		Sheet: sheet,
	}

	rawPayload, err := rlp.EncodeToBytes(pongPayload)
	if err != nil {
		return false
	}

	resp := &message.Message{
		Header:  NewHeader(w.host, w.network, uuid, message.MessageType_PONG),
		Payload: rawPayload,
	}

	// sign the data
	signature, err := Signature(w.host, resp)
	if err != nil {
		log.Debugf("failed to sign response")
		return false
	}

	// add the signature to the message
	resp.Header.Sign = signature

	// send the response
	err = Reply(stream, resp)
	if err != nil {
		log.Debugf("failed sending sheet to: %s: %s", stream.Conn().RemotePeer(), err)
		return false
	}

	log.Debugf("sent sheet to: %s with message id: %x", stream.Conn().RemotePeer(), resp.Header.MessageId)

	return true
}

// DecodeSheetPayload unmarshal the raw and return the *message.PongPayload.
func DecodeSheetPayload(rawPayload []byte) (*message.SheetPayload, error) {
	sheetPayload := &message.SheetPayload{}

	err := rlp.DecodeBytes(rawPayload, sheetPayload)
	if err != nil {
		return nil, err
	}

	return sheetPayload, nil
}
