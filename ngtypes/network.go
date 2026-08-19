package ngtypes

import "github.com/pkg/errors"

// Network is the type of the ngchain network
type Network uint8

// ErrNetworkInvalid is returned by ParseNetwork for an unknown name
var ErrNetworkInvalid = errors.New("invalid network name")

const (
	// ZERONET is the local regression testnet
	ZERONET Network = 0
	// TESTNET is the public internet testnet
	TESTNET Network = 1
	// MAINNET is the public network for production
	MAINNET Network = 2
)

// ParseNetwork converts a network name to a Network, erroring (never
// panicking) on an unknown name — for decoding UNTRUSTED input like a
// JSON tx/block, where an attacker-chosen "network":"FOO" must degrade to
// an error, not crash the node
func ParseNetwork(netName string) (Network, error) {
	switch netName {
	case "ZERONET":
		return ZERONET, nil
	case "TESTNET":
		return TESTNET, nil
	case "MAINNET":
		return MAINNET, nil
	default:
		return 0, errors.Wrapf(ErrNetworkInvalid, "%q", netName)
	}
}

// GetNetwork converts the network name to the Network type, panicking on
// an unknown name — for trusted, hardcoded call sites only
func GetNetwork(netName string) Network {
	net, err := ParseNetwork(netName)
	if err != nil {
		panic(err)
	}
	return net
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
