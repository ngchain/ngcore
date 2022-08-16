package ngtypes

// Network is the type of the ngchain network
type Network uint8

const (
	// ZERONET is the local regression testnet
	ZERONET Network = 0
	// TESTNET is the public internet testnet
	TESTNET Network = 1
	// MAINNET is the public network for production
	MAINNET Network = 2
)

// GetNetwork converts the network name to the Network type
func GetNetwork(netName string) Network {
	switch netName {
	case "ZERONET":
		return ZERONET
	case "TESTNET":
		return TESTNET
	case "MAINNET":
		return MAINNET
	default:
		panic("invalid network: " + netName)
	}
}

func (net Network) String() string {
	switch net {
	case ZERONET:
		return "ZERONET"
	case TESTNET:
		return "TESTNET"
	case MAINNET:
		return "MAINNET"
	default:
		panic("invalid network")
	}
}
