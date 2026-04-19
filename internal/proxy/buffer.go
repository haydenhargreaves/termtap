package proxy

import (
	"bufio"
	"io"
	"net"
)

type bufferedConn struct {
	net.Conn
	reader io.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func wrapBufferedConn(conn net.Conn, readWriter *bufio.ReadWriter) net.Conn {
	if readWriter == nil {
		return conn
	}

	return &bufferedConn{Conn: conn, reader: readWriter}
}

type previewReadCloser struct {
	io.ReadCloser
	preview *bodyPreview
}

func (p *previewReadCloser) Read(data []byte) (int, error) {
	n, err := p.ReadCloser.Read(data)
	if n > 0 {
		p.preview.Write(data[:n])
	}
	return n, err
}
