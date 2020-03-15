package ngp2p

import (
	"github.com/ngin-network/ngcore/ngtypes"
)

// TODO: maintain a RemoteNode Pool
type RemoteNode struct {
	*ngtypes.PingPongPayload
}
