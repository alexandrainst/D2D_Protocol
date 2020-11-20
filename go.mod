module alexandra.dk/D2D_Agent

go 1.15

replace github.com/alexandrainst/D2D-communication => /Users/skov/Alexandra/Alexandra/D2D/src/comm

replace github.com/alexandrainst/agentlogic => /Users/skov/Alexandra/Alexandra/D2D/src/agentlogic

require (
	github.com/alexandrainst/D2D-communication v0.9.1
	github.com/alexandrainst/agentlogic v0.2.0
	github.com/jinzhu/copier v0.0.0-20201025035756-632e723a6687
	github.com/libp2p/go-libp2p v0.11.0 // indirect
	github.com/libp2p/go-libp2p-core v0.6.1 // indirect
	github.com/libp2p/go-libp2p-pubsub v0.3.6 // indirect
	github.com/libp2p/go-libp2p-tls v0.1.3 // indirect
	github.com/multiformats/go-multiaddr v0.3.1 // indirect
	github.com/paulmach/orb v0.1.7
)
