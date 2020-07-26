// Package ngp2p is the ngin p2p protocol implement based on libp2p
//
// ngp2p uses libp2p(ipfs)'s dht for public peer discovery and mDNS for private, and uses pub-sub to work the broadcast net
// 
// the main peer-to-peer communication is based on Wired Protocol, which uses fast protobuf to encode and decode objects
package ngp2p
