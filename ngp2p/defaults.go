package ngp2p

const MaxBlocks = 1000

// pattern: /ngp2p/protocol-name/version
const (
	protocolVersion      = "0.0.3"
	WiredProtocol        = "/ngp2p/wired/" + protocolVersion
	DHTProtocolExtension = "/ngp2p/dht/" + protocolVersion
	broadcastBlockTopic  = "/ngp2p/broadcast/block/" + protocolVersion
	broadcastTxTopic     = "/ngp2p/broadcast/tx/" + protocolVersion
)
