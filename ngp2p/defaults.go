package ngp2p

// MaxBlocks limits the max number of blocks which are transfered on p2p network
const MaxBlocks = 1000

// pattern: /ngp2p/protocol-name/version
const (
	protocolVersion      = "0.0.6"
	WiredProtocol        = "/ngp2p/wired/" + protocolVersion
	DHTProtocolExtension = "/ngp2p/dht/" + protocolVersion
	broadcastBlockTopic  = "/ngp2p/broadcast/block/" + protocolVersion
	broadcastTxTopic     = "/ngp2p/broadcast/tx/" + protocolVersion
)
