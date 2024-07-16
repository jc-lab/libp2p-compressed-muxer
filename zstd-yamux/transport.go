package zstd_yamux

import (
	compressed_muxer "github.com/jc-lab/libp2p-compressed-muxer"
	"github.com/klauspost/compress/zstd"
	"github.com/libp2p/go-libp2p/p2p/muxer/yamux"
	"io"
)

const ID = "/zstd-yamux/1.0.0"

var DefaultTransport *compressed_muxer.Transport

func init() {
	DefaultTransport = compressed_muxer.NewTransport(yamux.DefaultTransport, &zstdCompressor{})
}

type zstdCompressor struct{}

func (z *zstdCompressor) NewEncoder(w io.Writer) (compressed_muxer.CompressEncoder, error) {
	return zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.SpeedFastest))
}

func (z *zstdCompressor) NewDecoder(r io.Reader) (io.Reader, error) {
	return zstd.NewReader(r)
}
