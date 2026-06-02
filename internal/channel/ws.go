// Package channel — bare-bones WebSocket client for Discord Gateway.
// Uses net/http and crypto/tls, no external WebSocket library.
// Implements RFC 6455 frame encoding/decoding at the level needed for Discord.

package channel

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
)

// ── WebSocket constants (RFC 6455) ──────────────────────────────────────────

const (
	wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

	wsOpContinuation = 0
	wsOpText         = 1
	wsOpClose        = 8
)

// ── discordWSConn ───────────────────────────────────────────────────────────

// discordWSConn is a minimal WebSocket connection for the Discord Gateway.
type discordWSConn struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex // guards writes
}

// dialWS opens a TLS WebSocket connection using only net/http helpers.
func dialWS(ctx context.Context, rawURL string) (*discordWSConn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":443"
	}

	dialer := &tls.Dialer{Config: &tls.Config{MinVersion: tls.VersionTLS12}}
	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, err
	}

	// Build WebSocket upgrade request
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		conn.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(nonce)

	req := fmt.Sprintf(
		"GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		u.RequestURI(), u.Hostname(), key,
	)

	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}

	// Read HTTP upgrade response
	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ws: read status line: %w", err)
	}

	if !strings.Contains(statusLine, "101") {
		// Read the rest of the response for diagnostics
		headers := ""
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			headers += line
			if line == "\r\n" {
				break
			}
		}
		conn.Close()

		// Try to parse as a standard HTTP error response
		if strings.Contains(statusLine, "401") {
			return nil, fmt.Errorf("discord: unauthorized — check bot token")
		}
		if strings.Contains(statusLine, "403") {
			return nil, fmt.Errorf("discord: forbidden — insufficient intents or permissions")
		}
		if strings.Contains(statusLine, "404") {
			return nil, fmt.Errorf("discord: not found")
		}
		return nil, fmt.Errorf("ws: unexpected status: %s; headers: %s", strings.TrimSpace(statusLine), headers)
	}

	// Read and validate the upgrade response headers
	gotAccept := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("ws: read headers: %w", err)
		}
		if line == "\r\n" {
			break
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "sec-websocket-accept:") {
			expectedAccept := computeAcceptKey(key)
			got := strings.TrimSpace(line[len("Sec-WebSocket-Accept:"):])
			got = strings.TrimSpace(strings.TrimSuffix(got, "\r\n"))
			if got != expectedAccept {
				conn.Close()
				return nil, fmt.Errorf("ws: accept key mismatch: got %q, want %q", got, expectedAccept)
			}
			gotAccept = true
		}
	}

	if !gotAccept {
		conn.Close()
		return nil, fmt.Errorf("ws: no Sec-WebSocket-Accept header")
	}

	return &discordWSConn{conn: conn, reader: reader}, nil
}

// computeAcceptKey computes the Sec-WebSocket-Accept header per RFC 6455 §4.2.2.
func computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ── Discord-specific helpers ────────────────────────────────────────────────

// readHello reads the initial HELLO frame (op 10) from Discord Gateway.
func (c *discordWSConn) readHello(ctx context.Context) (*discordHello, error) {
	msg, err := c.readMsg(ctx)
	if err != nil {
		return nil, err
	}
	if msg.Op != discordOpHello {
		return nil, fmt.Errorf("discord: expected op 10 (HELLO), got op %d", msg.Op)
	}

	var hello discordHello
	if err := json.Unmarshal(msg.D, &hello); err != nil {
		return nil, fmt.Errorf("discord: parse HELLO: %w", err)
	}
	return &hello, nil
}

// readMsg reads the next complete WebSocket text frame and returns it as a discordWSMsg.
func (c *discordWSConn) readMsg(ctx context.Context) (*discordWSMsg, error) {
	for {
		frame, err := c.readFrame(ctx)
		if err != nil {
			return nil, err
		}

		switch frame.opcode {
		case wsOpText:
			var msg discordWSMsg
			if err := json.Unmarshal(frame.payload, &msg); err != nil {
				return nil, fmt.Errorf("ws: parse message: %w", err)
			}
			return &msg, nil
		case wsOpClose:
			return nil, io.EOF
		case 9: // Ping — respond with Pong
			if err := c.writePong(frame.payload); err != nil {
				return nil, err
			}
			continue
		default:
			continue
		}
	}
}

// writePong sends a WebSocket Pong frame.
func (c *discordWSConn) writePong(payload []byte) error {
	return c.writeFrame(0xA, payload) // opcode 0xA = Pong
}

// writeMsg sends a JSON-encoded Discord Gateway message.
func (c *discordWSConn) writeMsg(op int, data json.RawMessage) error {
	msg := discordWSMsg{Op: op, D: data}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.writeFrame(wsOpText, body)
}

// ── Frame-level primitives ──────────────────────────────────────────────────

type wsFrame struct {
	opcode  int
	payload []byte
}

// readFrame reads a single WebSocket frame per RFC 6455 §5.2.
func (c *discordWSConn) readFrame(ctx context.Context) (*wsFrame, error) {
	// Read first 2 bytes
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, header); err != nil {
		if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "closed") {
			return nil, io.EOF
		}
		return nil, err
	}

	opcode := int(header[0] & 0x0F)
	masked := (header[1] & 0x80) != 0
	length := int64(header[1] & 0x7F)

	switch {
	case length == 126:
		buf := make([]byte, 2)
		if _, err := io.ReadFull(c.reader, buf); err != nil {
			return nil, err
		}
		length = int64(binary.BigEndian.Uint16(buf))
	case length == 127:
		buf := make([]byte, 8)
		if _, err := io.ReadFull(c.reader, buf); err != nil {
			return nil, err
		}
		length = int64(binary.BigEndian.Uint64(buf))
	}

	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(c.reader, mask[:]); err != nil {
			return nil, err
		}
	}

	if length > 1<<20 { // 1 MB sanity cap
		return nil, fmt.Errorf("ws: frame too large: %d bytes", length)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return nil, err
	}

	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}

	return &wsFrame{opcode: opcode, payload: payload}, nil
}

// writeFrame sends a single WebSocket frame (client→server, always masked).
func (c *discordWSConn) writeFrame(opcode int, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}

	// Mask payload
	maskedPayload := make([]byte, len(payload))
	for i := range payload {
		maskedPayload[i] = payload[i] ^ mask[i%4]
	}

	// Build header
	var hdr []byte
	hdr = append(hdr, byte(0x80|opcode)) // FIN + opcode

	length := len(payload)
	switch {
	case length <= 125:
		hdr = append(hdr, byte(0x80|length)) // masked flag + length
	case length <= 65535:
		hdr = append(hdr, byte(0x80|126))
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(length))
		hdr = append(hdr, buf...)
	default:
		hdr = append(hdr, byte(0x80|127))
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(length))
		hdr = append(hdr, buf...)
	}

	hdr = append(hdr, mask[:]...)

	// Write header + masked payload
	if _, err := c.conn.Write(hdr); err != nil {
		return err
	}
	if _, err := c.conn.Write(maskedPayload); err != nil {
		return err
	}
	return nil
}

// close sends a close frame and closes the underlying connection.
func (c *discordWSConn) close() error {
	// Best-effort: send close frame
	c.writeFrame(wsOpClose, nil)
	return c.conn.Close()
}