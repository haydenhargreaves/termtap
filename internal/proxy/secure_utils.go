package proxy

import (
	"bufio"
	"io"
	"net"
)

const maxDiscardBodyBytes = 1 << 20

func writeConnectEstablished(conn net.Conn, readWriter *bufio.ReadWriter) error {
	if readWriter != nil {
		if _, err := readWriter.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			return err
		}
		return readWriter.Flush()
	}

	_, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	return err
}

func discardAndCloseBody(body io.ReadCloser) {
	if body == nil {
		return
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(body, maxDiscardBodyBytes))
	_ = body.Close()
}
