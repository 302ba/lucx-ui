// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package signature

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

// CaptureResult holds the I1-I5 packet strings captured from a real host.
// I1 is the QUIC Initial we send (carrying a TLS ClientHello with SNI=domain);
// I2-I5 are the server's reply packets read off the same UDP socket.
type CaptureResult struct {
	I1, I2, I3, I4, I5 string
}

// QUIC v1 Initial salt (RFC 9001).
var quicV1Salt = []byte{0x38, 0x76, 0x2c, 0xf7, 0xf5, 0x59, 0x34, 0xb3, 0x4d, 0x17, 0x9a, 0xe6, 0xa4, 0xc8, 0x0c, 0xad, 0xcc, 0xbb, 0x7f, 0x0a}

const (
	maxPackets    = 5    // I1-I5
	maxPacketSize = 1500 // truncate longer captures
	quicPort      = "443"
	readTimeout   = 5 * time.Second
	minPackets    = 2 // need at least our Initial + 1 reply
)

// Capture sends a QUIC v1 Initial (carrying a TLS ClientHello with SNI=domain)
// to the host on UDP 443, reads the server's reply packets, and returns them
// as AmneziaWG CPS strings (I1 = what we sent, I2-I5 = replies). Ported from
// hoaxisr/awg-manager internal/signature/capture.go — pure Go, no libpcap.
//
// Returns an error when the host doesn't speak QUIC (no reply within the
// timeout) — AWG can only mimic QUIC-fronted hosts. TLS capture is not
// supported (TLS signatures are incompatible with AWG and crash it, per
// hoaxisr).
func Capture(domain string) (CaptureResult, error) {
	host := normalizeDomain(domain)
	if host == "" {
		return CaptureResult{}, errors.New("signature: empty domain")
	}
	ip, err := resolveHost(host)
	if err != nil {
		return CaptureResult{}, fmt.Errorf("signature: resolve %q: %w", host, err)
	}
	packets, err := captureQUIC(host, ip)
	if err != nil {
		return CaptureResult{}, err
	}
	return fillPackets(packets), nil
}

// normalizeDomain strips scheme/path/port from the user input, leaving the
// bare domain (google.com from https://google.com:443/path).
func normalizeDomain(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil && u.Host != "" {
			return u.Hostname()
		}
	}
	if i := strings.LastIndexByte(s, ':'); i > 0 && !strings.Contains(s[i+1:], ".") {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '/'); i > 0 {
		s = s[:i]
	}
	return s
}

// resolveHost resolves a domain to a single IPv4 (AWG QUIC fronting is IPv4).
func resolveHost(domain string) (string, error) {
	ips, err := net.LookupIP(domain)
	if err != nil {
		return "", err
	}
	for _, ip := range ips {
		if ip.To4() != nil {
			return ip.To4().String(), nil
		}
	}
	if len(ips) > 0 {
		return ips[0].String(), nil
	}
	return "", errors.New("no IP found")
}

// captureQUIC dials UDP 443, sends a QUIC Initial (with a TLS ClientHello
// SNI=host), then reads reply packets. Returns the sent Initial first, then
// up to maxPackets-1 replies.
func captureQUIC(host, ip string) ([][]byte, error) {
	initial, err := buildQUICInitial(host)
	if err != nil {
		return nil, fmt.Errorf("signature: build QUIC initial: %w", err)
	}
	conn, err := net.DialTimeout("udp", net.JoinHostPort(ip, quicPort), readTimeout)
	if err != nil {
		return nil, fmt.Errorf("signature: dial %s:443: %w", ip, err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))

	if _, err := conn.Write(initial); err != nil {
		return nil, fmt.Errorf("signature: write initial: %w", err)
	}
	packets := [][]byte{initial}
	buf := make([]byte, 2048)
	for len(packets) < maxPackets {
		n, err := conn.Read(buf)
		if err != nil {
			break // timeout / closed: keep what we have
		}
		if n <= 0 {
			continue
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		packets = append(packets, pkt)
	}
	if len(packets) < minPackets {
		return nil, fmt.Errorf("signature: %s did not reply on QUIC 443 (got %d packet(s)) — host may not support QUIC", host, len(packets))
	}
	return packets, nil
}

// fillPackets converts raw packet bytes into AmneziaWG CPS "<b 0xHEX>" strings,
// truncating each to maxPacketSize. Fewer than 5 packets leave the rest empty.
func fillPackets(packets [][]byte) CaptureResult {
	res := CaptureResult{}
	fields := [5]*string{&res.I1, &res.I2, &res.I3, &res.I4, &res.I5}
	for i, pkt := range packets {
		if i >= maxPackets {
			break
		}
		if len(pkt) > maxPacketSize {
			pkt = pkt[:maxPacketSize]
		}
		*fields[i] = "<b 0x" + hex.EncodeToString(pkt) + ">"
	}
	return res
}

// buildQUICInitial assembles a QUIC v1 Long-Header Initial packet carrying a
// TLS 1.3 ClientHello (SNI=host), encrypts the CRYPTO frame payload with
// AES-128-GCM (initial keys derived from the random DCID via HKDF-SHA256 per
// RFC 9001 §5.2), and applies header protection (RFC 9001 §5.4).
func buildQUICInitial(host string) ([]byte, error) {
	ch, err := buildTLSClientHello(host)
	if err != nil {
		return nil, err
	}
	dcid := make([]byte, 8)
	if _, err := rand.Read(dcid); err != nil {
		return nil, err
	}
	scid := make([]byte, 8)
	if _, err := rand.Read(scid); err != nil {
		return nil, err
	}
	// CRYPTO frame (type 0x06, offset 0, length, ClientHello).
	var crypto bytes.Buffer
	crypto.WriteByte(0x06)
	crypto.Write(appendVarint(0))                // offset
	crypto.Write(appendVarint(len(ch)))          // length
	crypto.Write(ch)
	return buildInitialPacket(dcid, scid, crypto.Bytes()), nil
}

// buildInitialPacket builds the Long Header, derives initial keys from dcid,
// encrypts the payload, and applies header protection.
func buildInitialPacket(dcid, scid, payload []byte) []byte {
	const pnLen = 4
	var hdr bytes.Buffer
	hdr.WriteByte(0xC0 | byte(pnLen-1))          // 0xC3 (long, 4-byte pn)
	hdr.Write([]byte{0x00, 0x00, 0x00, 0x01})    // QUIC v1
	hdr.WriteByte(byte(len(dcid)))
	hdr.Write(dcid)
	hdr.WriteByte(byte(len(scid)))
	hdr.Write(scid)
	hdr.Write(appendVarint(0))                   // token length = 0
	pn := make([]byte, pnLen)                    // packet number 0
	// Pad so the full packet reaches ~1200 bytes (QUIC minimum Initial).
	lengthVal := pnLen + len(payload)
	needed := 1200 - hdr.Len() - varintLen(lengthVal) - pnLen
	if needed > len(payload) {
		pad := needed - len(payload)
		padded := make([]byte, len(payload)+pad)
		copy(padded, payload)
		payload = padded
		lengthVal = pnLen + len(payload)
	}
	hdr.Write(appendVarint(lengthVal))
	hdr.Write(pn) // unprotected pn (masked later)

	// Derive initial keys from dcid (RFC 9001 §5.2).
	initialSecret, _ := hkdf.Extract(sha256.New, dcid, quicV1Salt)
	clientSecret := hkdfExpandLabel(initialSecret, "client in", nil, 32)
	key := hkdfExpandLabel(clientSecret, "quic key", nil, 16)  // AES-128
	iv := hkdfExpandLabel(clientSecret, "quic iv", nil, 12)
	hp := hkdfExpandLabel(clientSecret, "quic hp", nil, 16)    // header protection

	// Nonce = iv XOR packet_number (pn=0 → nonce = iv).
	nonce := make([]byte, 12)
	copy(nonce, iv)

	// AES-128-GCM: AAD = unprotected header (without pn), plaintext = pn + payload.
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	aad := hdr.Bytes()[:hdr.Len()-pnLen] // header up to but not including pn
	plaintext := append(append([]byte{}, pn...), payload...)
	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)

	// Header protection (RFC 9001 §5.4): mask = AES-ECB(hp, sample), sample =
	// first 16 bytes of ciphertext. XOR mask into the form byte (low 4 bits)
	// and the pn bytes.
	protected := make([]byte, 0, len(aad)+len(ciphertext))
	protected = append(protected, aad...)
	protected = append(protected, ciphertext...)
	if len(ciphertext) >= 16 {
		mask := aesECB(hp, ciphertext[:16])
		pnOffset := len(aad) - pnLen // position of pn in the buffer
		protected[pnOffset] ^= mask[0] & 0x0F
		for i := 0; i < pnLen; i++ {
			protected[pnOffset+1+i] ^= mask[1+i]
		}
	}
	return protected
}

// buildTLSClientHello produces a raw TLS 1.3 ClientHello (with SNI=host, ALPN
// h3) using crypto/tls via a net.Pipe: we drive a TLS client whose
// ServerName=host and ALPN=["h3"], read the raw ClientHello bytes from the
// pipe before the (expected) handshake failure. Ported from hoaxisr.
func buildTLSClientHello(host string) ([]byte, error) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	var captured bytes.Buffer
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(&captured, serverConn)
		done <- err
	}()

	tlsConf := &tls.Config{
		ServerName:         host,
		NextProtos:         []string{"h3"},
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
	}
	tlsConn := tls.Client(clientConn, tlsConf)
	_ = tlsConn.SetDeadline(time.Now().Add(2 * time.Second))
	_ = tlsConn.Handshake() // fails — server side is just a pipe
	clientConn.Close()
	<-done

	ch := captured.Bytes()
	if len(ch) < 10 {
		return nil, fmt.Errorf("signature: TLS ClientHello capture too short (%d bytes)", len(ch))
	}
	return ch, nil
}

// hkdfExpandLabel implements HKDF-Expand-Label (RFC 8446 §7.1) with the TLS
// 1.3 "tls13 " label prefix.
func hkdfExpandLabel(secret []byte, label string, context []byte, length int) []byte {
	fullLabel := "tls13 " + label
	var info bytes.Buffer
	binary.Write(&info, binary.BigEndian, uint16(length))
	info.WriteByte(byte(len(fullLabel)))
	info.WriteString(fullLabel)
	info.WriteByte(byte(len(context)))
	info.Write(context)
	out, _ := hkdf.Expand(sha256.New, secret, string(info.Bytes()), length)
	return out
}

// appendVarint writes a QUIC variable-length integer (RFC 9000 §16) using the
// minimal encoding.
func appendVarint(v int) []byte {
	switch {
	case v < 64:
		return []byte{byte(v)}
	case v < 16384:
		return []byte{byte(v>>8) | 0x40, byte(v)}
	case v < 1073741824:
		return []byte{byte(v>>24) | 0x80, byte(v >> 16), byte(v >> 8), byte(v)}
	default:
		return []byte{byte(v>>56) | 0xC0, byte(v >> 48), byte(v >> 40), byte(v >> 32), byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
}

// varintLen returns the byte length of the varint encoding for v.
func varintLen(v int) int {
	switch {
	case v < 64:
		return 1
	case v < 16384:
		return 2
	case v < 1073741824:
		return 4
	default:
		return 8
	}
}

// aesECB encrypts a single 16-byte block with AES-ECB (QUIC header protection mask).
func aesECB(key, block []byte) []byte {
	c, _ := aes.NewCipher(key)
	out := make([]byte, len(block))
	c.Encrypt(out, block)
	return out
}