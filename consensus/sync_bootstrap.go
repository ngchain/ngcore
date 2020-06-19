package consensus

import (
	"sort"

	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/multiformats/go-multiaddr"

	"github.com/ngchain/ngcore/ngp2p"
	"github.com/ngchain/ngcore/storage"
)

func (sync *syncModule) bootstrap() {
	c := context.Background()

	peerStore := ngp2p.GetLocalNode().Peerstore()

	okNum := 0
	for i := range ngp2p.BootstrapNodes {
		addr, err := multiaddr.NewMultiaddr(ngp2p.BootstrapNodes[i])
		if err != nil {
			panic(err)
		}

		p, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			panic(err)
		}

		err = ngp2p.GetLocalNode().Connect(c, *p)
		if err != nil {
			log.Error(err)
		} else {
			okNum++
		}

		peerStore.AddAddr(p.ID, addr, peerstore.PermanentAddrTTL)
	}

	if okNum == 0 {
		panic("all bootstrap nodes are unreachable, check your net status")
	}

	for _, id := range peerStore.Peers() {
		if id != ngp2p.GetLocalNode().ID() {
			err := sync.getRemoteStatus(id)
			if err != nil {
				log.Error(err)
			}
		}
	}

	slice := make([]*remoteRecord, len(sync.store))
	i := 0

	for _, v := range sync.store {
		slice[i] = v
		i++
	}

	sort.SliceStable(slice, func(i, j int) bool {
		return slice[i].lastChatTime > slice[j].lastChatTime
	})

	// initial sync
	for _, r := range slice {
		if r.shouldSync() {
			err := sync.doInit(r)
			if err != nil {
				panic(err)
			}
		}
	}
}

func (sync *syncModule) doInit(record *remoteRecord) error {
	sync.Lock()
	defer sync.Unlock()

	log.Warnf("start initial syncing with remote node %s", record.id)

	// get chain
	for storage.GetChain().GetLatestBlockHeight() < record.latest {
		chain, err := sync.getRemoteChain(record.id)
		if err != nil {
			return err
		}

		for i := 0; i < len(chain); i++ {
			err = storage.GetChain().PutNewBlock(chain[i])
			if err != nil {
				return err
			}
		}
	}

	return nil
}
