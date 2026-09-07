package lyresdk

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func serverFrame(op byte, fin bool, payload []byte) []byte {
	if fin {
		op |= 128
	}
	b := []byte{op}
	switch {
	case len(payload) < 126:
		b = append(b, byte(len(payload)))
	case len(payload) <= 65535:
		b = append(b, 126, byte(len(payload)>>8), byte(len(payload)))
	default:
		b = append(b, 127)
		b = binary.BigEndian.AppendUint64(b, uint64(len(payload)))
	}
	return append(b, payload...)
}
func readClientFrame(r io.Reader) (byte, []byte, error) {
	var head [2]byte
	if _, e := io.ReadFull(r, head[:]); e != nil {
		return 0, nil, e
	}
	if head[0]&128 == 0 || head[1]&128 == 0 {
		return 0, nil, fmt.Errorf("client frame must be final and masked")
	}
	n := uint64(head[1] & 127)
	switch n {
	case 126:
		var x [2]byte
		if _, e := io.ReadFull(r, x[:]); e != nil {
			return 0, nil, e
		}
		n = uint64(binary.BigEndian.Uint16(x[:]))
	case 127:
		var x [8]byte
		if _, e := io.ReadFull(r, x[:]); e != nil {
			return 0, nil, e
		}
		n = binary.BigEndian.Uint64(x[:])
	}
	if n > maxFrameBytes {
		return 0, nil, fmt.Errorf("too large")
	}
	var mask [4]byte
	if _, e := io.ReadFull(r, mask[:]); e != nil {
		return 0, nil, e
	}
	b := make([]byte, n)
	_, e := io.ReadFull(r, b)
	for i := range b {
		b[i] ^= mask[i%4]
	}
	return head[0] & 15, b, e
}
func TestWebSocketLengthsAndMasking(t *testing.T) {
	for _, n := range []int{0, 1, 125, 126, 127, 128, 65535, 65536} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			_ = b.SetDeadline(time.Now().Add(3 * time.Second))
			c := &websocketStream{Conn: a, reader: bufio.NewReader(a), writeTimeout: time.Second}
			want := bytes.Repeat([]byte{23}, n)
			done := make(chan struct{})
			go func() {
				defer close(done)
				op, got, e := readClientFrame(b)
				if e != nil || op != 2 || !bytes.Equal(got, want) {
					t.Errorf("client frame %d %v", op, e)
				}
				if e = writeAll(b, serverFrame(2, true, want)); e != nil {
					t.Error(e)
				}
				_ = writeAll(b, serverFrame(2, true, []byte{99}))
			}()
			if _, e := c.Write(want); e != nil {
				t.Fatal(e)
			}
			got := make([]byte, n+1)
			if _, e := io.ReadFull(c, got); e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(got, append(want, 99)) {
				t.Error("stream differs")
			}
			<-done
		})
	}
}
func TestWebSocketFragmentsAndPing(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	_ = b.SetDeadline(time.Now().Add(3 * time.Second))
	c := &websocketStream{Conn: a, reader: bufio.NewReader(a), writeTimeout: time.Second}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = writeAll(b, serverFrame(2, false, []byte("ab")))
		_ = writeAll(b, serverFrame(9, true, []byte("ping")))
		op, got, e := readClientFrame(b)
		if e != nil || op != 10 || string(got) != "ping" {
			t.Errorf("pong %d %q %v", op, got, e)
		}
		_ = writeAll(b, serverFrame(0, true, []byte("cd")))
	}()
	got := make([]byte, 4)
	if _, e := io.ReadFull(c, got); e != nil {
		t.Fatal(e)
	}
	if string(got) != "abcd" {
		t.Error(string(got))
	}
	<-done
}
func TestWebSocketRejectsInvalidFrames(t *testing.T) {
	for name, frame := range map[string][]byte{"masked": {130, 128}, "text": {129, 0}, "continuation": {128, 0}, "rsv": {194, 0}, "nonminimal": {130, 126, 0, 1}, "fragmented control": {9, 0}, "bad close": {136, 2, 3, 237}, "too large": append([]byte{130, 127}, binary.BigEndian.AppendUint64(nil, maxFrameBytes+1)...)} {
		t.Run(name, func(t *testing.T) {
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			c := &websocketStream{Conn: a, reader: bufio.NewReader(a), writeTimeout: time.Second}
			go writeAll(b, frame)
			if _, e := c.Read(make([]byte, 1)); e == nil {
				t.Error("accepted invalid frame")
			}
		})
	}
}
func TestWebSocketHandshakeTLS(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/service/ws" {
			t.Error("path lost")
		}
		sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		conn, rw, e := w.(http.Hijacker).Hijack()
		if e != nil {
			t.Error(e)
			return
		}
		defer conn.Close()
		fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nConnection: keep-alive, Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(sum[:]))
		rw.Write(serverFrame(2, true, []byte("ready")))
		rw.Flush()
	})
	srv := httptest.NewTLSServer(handler)
	defer srv.Close()
	url := "wss" + strings.TrimPrefix(srv.URL, "https") + "/service/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if c, e := DialTransport(ctx, url, TransportOptions{}); e == nil {
		c.Close()
		t.Fatal("trusted self-signed certificate")
	}
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	c, e := DialTransport(ctx, url, TransportOptions{TLSConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	got := make([]byte, 5)
	if _, e = io.ReadFull(c, got); e != nil || string(got) != "ready" {
		t.Fatalf("%q %v", got, e)
	}
}
func TestWebSocketRejectsBadHandshake(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Connection", "upgrade")
		w.Header().Set("Upgrade", "websocket")
		w.WriteHeader(101)
	}))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if c, e := DialTransport(ctx, "ws"+strings.TrimPrefix(s.URL, "http"), TransportOptions{}); e == nil {
		c.Close()
		t.Fatal("accepted missing accept proof")
	}
}
