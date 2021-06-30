package ngtypes

type Network uint8

const (
	ZERONET Network = 0
	TESTNET Network = 1
	MAINNET Network = 2
)

func GetNetwork(netName string) Network {
	switch netName {
	case "ZERONET":
		return ZERONET
	case "TESTNET":
		return TESTNET
	case "MAINNET":
		return MAINNET
	default:
		panic("invalid network")
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
