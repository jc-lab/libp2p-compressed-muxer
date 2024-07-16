package compressed_muxer

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type CompressEncoder interface {
	io.Writer
	Flush() error
}

type CompressorFactory interface {
	NewEncoder(w io.Writer) (CompressEncoder, error)
	NewDecoder(r io.Reader) (io.Reader, error)
}

type CompressMetrics interface {
	GetAll() (netRead int64, netWrite int64, unCompRead int64, unCompWrite int64)
	GetNetRead() int64
	GetNetWrite() int64
	GetUnCompRead() int64
	GetUnCompWrite() int64
}

type compNetConn struct {
	net.Conn
	encoder CompressEncoder
	decoder io.Reader

	netRead     int64
	unCompRead  int64
	netWrite    int64
	unCompWrite int64

	flushMutex  sync.Mutex
	flushPeriod time.Duration
	flushing    bool
	flushTimer  *time.Timer
	flushErr    error
	closeCh     chan any
}

func (c *compNetConn) Read(b []byte) (int, error) {
	n, err := c.decoder.Read(b)
	atomic.AddInt64(&c.unCompRead, int64(n))
	return n, err
}

func (c *compNetConn) Write(b []byte) (int, error) {
	c.flushMutex.Lock()
	defer c.flushMutex.Unlock()

	if c.flushErr != nil {
		return 0, c.flushErr
	}

	n, err := c.encoder.Write(b)
	atomic.AddInt64(&c.unCompWrite, int64(n))
	if n > 0 {
		if !c.flushing {
			c.flushing = true
			c.flushTimer.Reset(c.flushPeriod)
		}
	}
	return n, err
}

func (c *compNetConn) Close() error {
	c.flushTimer.Stop()
	close(c.closeCh)
	return c.Conn.Close()
}

func (c *compNetConn) GetAll() (netRead int64, netWrite int64, unCompRead int64, unCompWrite int64) {
	return atomic.LoadInt64(&c.netRead), atomic.LoadInt64(&c.netWrite), atomic.LoadInt64(&c.unCompRead), atomic.LoadInt64(&c.unCompWrite)
}

func (c *compNetConn) GetNetRead() int64 {
	return atomic.LoadInt64(&c.netRead)
}

func (c *compNetConn) GetNetWrite() int64 {
	return atomic.LoadInt64(&c.netWrite)
}

func (c *compNetConn) GetUnCompRead() int64 {
	return atomic.LoadInt64(&c.unCompRead)
}

func (c *compNetConn) GetUnCompWrite() int64 {
	return atomic.LoadInt64(&c.unCompWrite)
}

func (c *compNetConn) flushWorker() {
	for {
		select {
		case _ = <-c.flushTimer.C:
			c.doFlush()
		case _, _ = <-c.closeCh:
			return
		}
	}
}

func (c *compNetConn) doFlush() {
	c.flushMutex.Lock()
	defer c.flushMutex.Unlock()
	c.flushing = false
	c.flushErr = c.encoder.Flush()
}

func wrapConn(parent net.Conn, compressor CompressorFactory) (net.Conn, error) {
	var err error

	wrappedConn := &compNetConn{
		Conn:        parent,
		flushPeriod: time.Microsecond,
		closeCh:     make(chan any),
	}

	wrappedConn.encoder, err = compressor.NewEncoder(&counterWriter{
		parent:  parent,
		counter: &wrappedConn.netWrite,
	})
	if err != nil {
		return nil, err
	}
	wrappedConn.decoder, err = compressor.NewDecoder(&counterReader{
		parent:  parent,
		counter: &wrappedConn.netRead,
	})
	if err != nil {
		return nil, err
	}

	wrappedConn.flushTimer = time.NewTimer(time.Second)
	wrappedConn.flushTimer.Stop()
	go wrappedConn.flushWorker()

	return wrappedConn, nil
}

type counterReader struct {
	parent  io.Reader
	counter *int64
}

func (r *counterReader) Read(b []byte) (int, error) {
	n, err := r.parent.Read(b)
	if n > 0 {
		atomic.AddInt64(r.counter, int64(n))
	}
	return n, err
}

type counterWriter struct {
	parent  io.Writer
	counter *int64
}

func (w *counterWriter) Write(p []byte) (int, error) {
	n, err := w.parent.Write(p)
	if n > 0 {
		atomic.AddInt64(w.counter, int64(n))
	}
	return n, err
}
