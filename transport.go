package compressed_muxer

import (
	"github.com/libp2p/go-libp2p/core/network"
	"net"
)

type Transport struct {
	multiplexer network.Multiplexer
	compressor  CompressorFactory
}

func NewTransport(multiplexer network.Multiplexer, compressor CompressorFactory) *Transport {
	return &Transport{
		multiplexer: multiplexer,
		compressor:  compressor,
	}
}

func (t *Transport) NewConn(nc net.Conn, isServer bool, scope network.PeerScope) (network.MuxedConn, error) {
	wc, err := wrapConn(nc, t.compressor)
	if err != nil {
		return nil, err
	}

	return t.multiplexer.NewConn(wc, isServer, scope)
}
