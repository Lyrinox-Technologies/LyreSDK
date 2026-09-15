package lyresdk

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Lyrinox-Technologies/ridged-proto/rdgproto"
)

const maxFrameBytes = rdgproto.MaxPayloadSize + 2048

// TransportOptions applies to the standard-library TCP, TLS and WebSocket dialers.
type TransportOptions struct {
	TLSConfig    *tls.Config
	Headers      http.Header
	WriteTimeout time.Duration
}

// DialTransport returns an ordinary RDGProto byte stream. ws/wss adapt the
// current Lyre deployment; tcp/tls use RDGProto's native stream framing directly.
func DialTransport(ctx context.Context, address string, options TransportOptions) (rdgproto.Connection, error) {
	u, err := url.Parse(address)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return nil, errors.New("invalid Lyre transport URL")
	}
	secure := u.Scheme == "wss" || u.Scheme == "tls"
	if u.Scheme != "ws" && u.Scheme != "wss" && u.Scheme != "tcp" && u.Scheme != "tls" {
		return nil, errors.New("transport must be ws, wss, tcp, or tls")
	}
	if (u.Scheme == "tcp" || u.Scheme == "tls") && (u.Path != "" || u.RawQuery != "") {
		return nil, errors.New("raw transport URLs cannot have a path or query")
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "tcp" || u.Scheme == "tls" {
			return nil, errors.New("raw transport requires a port")
		}
		port = "80"
		if secure {
			port = "443"
		}
	}
	netAddress := net.JoinHostPort(u.Hostname(), port)
	dialer := net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	var conn net.Conn
	if secure {
		cfg := &tls.Config{MinVersion: tls.VersionTLS12}
		if options.TLSConfig != nil {
			cfg = options.TLSConfig.Clone()
		}
		if cfg.ServerName == "" {
			cfg.ServerName = u.Hostname()
		}
		conn, err = (&tls.Dialer{NetDialer: &dialer, Config: cfg}).DialContext(ctx, "tcp", netAddress)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", netAddress)
	}
	if err != nil {
		return nil, err
	}
	if u.Scheme == "tcp" || u.Scheme == "tls" {
		return conn, nil
	}
	success := false
	defer func() {
		if !success {
			conn.Close()
		}
	}()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline := time.Now().Add(30 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(nonce[:])
	headers := options.Headers.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	for _, name := range []string{"Connection", "Upgrade", "Sec-WebSocket-Key", "Sec-WebSocket-Version", "Sec-WebSocket-Extensions", "Sec-WebSocket-Protocol"} {
		if headers.Get(name) != "" {
			return nil, fmt.Errorf("transport owns header %s", name)
		}
	}
	headers.Set("Connection", "Upgrade")
	headers.Set("Upgrade", "websocket")
	headers.Set("Sec-WebSocket-Version", "13")
	headers.Set("Sec-WebSocket-Key", key)
	request := &http.Request{Method: http.MethodGet, URL: u, Host: u.Host, Header: headers}
	if err = request.Write(conn); err != nil {
		return nil, err
	}
	limited := &io.LimitedReader{R: conn, N: 64 << 10}
	reader := bufio.NewReader(limited)
	response, err := http.ReadResponse(reader, request)
	if err != nil {
		return nil, fmt.Errorf("WebSocket handshake: %w", err)
	}
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	if response.StatusCode != 101 || !headerToken(response.Header.Get("Connection"), "upgrade") || !strings.EqualFold(response.Header.Get("Upgrade"), "websocket") || response.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(sum[:]) || response.Header.Get("Sec-WebSocket-Extensions") != "" || response.Header.Get("Sec-WebSocket-Protocol") != "" {
		return nil, fmt.Errorf("WebSocket upgrade rejected (HTTP %d)", response.StatusCode)
	}
	limited.N = math.MaxInt64
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	writeTimeout := options.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = 30 * time.Second
	}
	success = true
	return &websocketStream{Conn: conn, reader: reader, writeTimeout: writeTimeout}, nil
}
func headerToken(value, want string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), want) {
			return true
		}
	}
	return false
}

// websocketStream implements RFC 6455 binary framing without a third-party
// transport. Only a single reader is used; control writes share the write lock.
type websocketStream struct {
	net.Conn
	reader       *bufio.Reader
	writeMu      sync.Mutex
	writeTimeout time.Duration
	remaining    uint64
	fragmented   bool
	messageBytes uint64
}

func (c *websocketStream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if c.remaining > 0 {
			n := len(p)
			if uint64(n) > c.remaining {
				n = int(c.remaining)
			}
			got, err := io.ReadFull(c.reader, p[:n])
			c.remaining -= uint64(got)
			return got, err
		}
		var head [2]byte
		if _, err := io.ReadFull(c.reader, head[:]); err != nil {
			return 0, err
		}
		fin := head[0]&0x80 != 0
		op := head[0] & 15
		if head[0]&0x70 != 0 || head[1]&0x80 != 0 {
			return 0, c.protocolError("invalid server frame flags")
		}
		size := uint64(head[1] & 127)
		if size == 126 {
			var b [2]byte
			if _, e := io.ReadFull(c.reader, b[:]); e != nil {
				return 0, e
			}
			size = uint64(binary.BigEndian.Uint16(b[:]))
			if size < 126 {
				return 0, c.protocolError("non-minimal frame length")
			}
		} else if size == 127 {
			var b [8]byte
			if _, e := io.ReadFull(c.reader, b[:]); e != nil {
				return 0, e
			}
			size = binary.BigEndian.Uint64(b[:])
			if size < 65536 || size>>63 != 0 {
				return 0, c.protocolError("invalid frame length")
			}
		}
		if size > maxFrameBytes {
			return 0, c.protocolError("WebSocket frame exceeds limit")
		}
		if op >= 8 {
			if !fin || size > 125 {
				return 0, c.protocolError("invalid control frame")
			}
			b := make([]byte, int(size))
			if _, e := io.ReadFull(c.reader, b); e != nil {
				return 0, e
			}
			switch op {
			case 8:
				if len(b) == 1 || (len(b) >= 2 && !validCloseCode(binary.BigEndian.Uint16(b[:2]))) || (len(b) > 2 && !utf8.Valid(b[2:])) {
					return 0, c.protocolError("invalid close frame")
				}
				_ = c.frame(8, b)
				c.Close()
				return 0, io.EOF
			case 9:
				if e := c.frame(10, b); e != nil {
					return 0, e
				}
			case 10:
			default:
				return 0, c.protocolError("unknown control opcode")
			}
			continue
		}
		switch op {
		case 2:
			if c.fragmented {
				return 0, c.protocolError("data frame during fragmented message")
			}
			c.messageBytes = 0
		case 0:
			if !c.fragmented {
				return 0, c.protocolError("unexpected continuation")
			}
		default:
			return 0, c.protocolError("Lyre transport requires binary frames")
		}
		c.messageBytes += size
		if c.messageBytes > maxFrameBytes {
			return 0, c.protocolError("fragmented message exceeds limit")
		}
		c.fragmented = !fin
		c.remaining = size
	}
}
func (c *websocketStream) protocolError(message string) error { c.Close(); return errors.New(message) }
func (c *websocketStream) Write(p []byte) (int, error) {
	if len(p) > maxFrameBytes {
		return 0, errors.New("frame exceeds limit")
	}
	if err := c.frame(2, p); err != nil {
		return 0, err
	}
	return len(p), nil
}
func (c *websocketStream) frame(op byte, p []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.Conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	defer c.Conn.SetWriteDeadline(time.Time{})
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}
	header := []byte{0x80 | op}
	switch {
	case len(p) < 126:
		header = append(header, 0x80|byte(len(p)))
	case len(p) <= 65535:
		header = append(header, 0xfe, byte(len(p)>>8), byte(len(p)))
	default:
		header = append(header, 0xff)
		header = binary.BigEndian.AppendUint64(header, uint64(len(p)))
	}
	header = append(header, mask[:]...)
	if err := writeAll(c.Conn, header); err != nil {
		return err
	}
	var chunk [32 << 10]byte
	for offset := 0; offset < len(p); {
		n := len(p) - offset
		if n > len(chunk) {
			n = len(chunk)
		}
		for i := 0; i < n; i++ {
			chunk[i] = p[offset+i] ^ mask[(offset+i)%4]
		}
		if err := writeAll(c.Conn, chunk[:n]); err != nil {
			return err
		}
		offset += n
	}
	return nil
}
func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}

func validCloseCode(code uint16) bool {
	return code >= 3000 && code <= 4999 || code >= 1000 && code <= 1014 && code != 1004 && code != 1005 && code != 1006
}
