// p2p/transport.go
// Copyright 2020 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package p2p

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary" // ★ 追加
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/bitutil"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/p2p/rlpx"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/golang/snappy"
)

const (
	// total timeout for encryption handshake and protocol
	// handshake in both directions.
	handshakeTimeout = 5 * time.Second

	// This is the timeout for sending the disconnect reason.
	// This is shorter than the usual timeout because we don't want
	// to wait if the connection is known to be bad anyway.
	discWriteTimeout = 1 * time.Second
)

type frameConn interface {
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	SetDeadline(t time.Time) error
	Read() (code uint64, data []byte, wireSize int, err error) // ← int
	Write(code uint64, data []byte) (uint32, error)            // ← uint32
	SetSnappy(enable bool)
	Close() error
}

// ------------------------------------------------------------
// rlpxTransport: 従来 *rlpx.Conn だけを持っていたのを、
// 生ソケット(raw) と frameConn(= rlpx.Conn or plainConn) の2本立てに変更
// ------------------------------------------------------------
type rlpxTransport struct {
	rmu, wmu    sync.Mutex
	wbuf        bytes.Buffer
	raw         net.Conn   // ★ 追加: 生の net.Conn
	conn        frameConn  // ★ 変更: *rlpx.Conn → frameConn
	rlp         *rlpx.Conn // ★ 追加: 本物の rlpx.Conn（ハンドシェイク時のみ使用）
	passthrough bool
}

func newRLPX(conn net.Conn, dialDest *ecdsa.PublicKey) transport {
	r := rlpx.NewConn(conn, dialDest)
	return &rlpxTransport{
		raw:  conn, // ★
		conn: r,    // ★ 最初は rlpx.Conn を使う
		rlp:  r,    // ★ Handshake 用に保持
	}
}

// transport.go（rlpxTransport の宣言付近）
type plainEnabler interface {
	ForcePlainFrames()
}

func (t *rlpxTransport) ForcePlainFrames() {
	// まずはメソッドを持っているかどうかを動的に確認
	if pe, ok := t.conn.(interface{ ForcePlainFrames() }); ok {
		pe.ForcePlainFrames()
		return
	}
	// 念のため: 具体型が *rlpx.Conn のときもハンドル（上と二重になるが安全）
	if rc, ok := t.conn.(*rlpx.Conn); ok {
		rc.ForcePlainFrames()
	}
	// どちらでもなければ何もしない（no-op）
}

func (t *rlpxTransport) ReadMsg() (Msg, error) {
	t.rmu.Lock()
	defer t.rmu.Unlock()

	var msg Msg
	t.conn.SetReadDeadline(time.Now().Add(frameReadTimeout))
	code, data, wireSize, err := t.conn.Read()
	if err == nil {
		// Protocol messages are dispatched to subprotocol handlers asynchronously,
		// but package rlpx may reuse the returned 'data' buffer on the next call
		// to Read. Copy the message data to avoid this being an issue.
		data = common.CopyBytes(data)
		msg = Msg{
			ReceivedAt: time.Now(),
			Code:       code,
			Size:       uint32(len(data)),
			meterSize:  uint32(wireSize),
			Payload:    bytes.NewReader(data),
		}
	}
	return msg, err
}

func (t *rlpxTransport) WriteMsg(msg Msg) error {
	t.wmu.Lock()
	defer t.wmu.Unlock()

	// Copy message data to write buffer.
	t.wbuf.Reset()
	if _, err := io.CopyN(&t.wbuf, msg.Payload, int64(msg.Size)); err != nil {
		return err
	}

	// Write the message.
	t.conn.SetWriteDeadline(time.Now().Add(frameWriteTimeout))
	size, err := t.conn.Write(msg.Code, t.wbuf.Bytes())
	if err != nil {
		return err
	}

	// Set metrics.
	msg.meterSize = size
	if metrics.Enabled && msg.meterCap.Name != "" { // don't meter non-subprotocol messages
		m := fmt.Sprintf("%s/%s/%d/%#02x", egressMeterName, msg.meterCap.Name, msg.meterCap.Version, msg.meterCode)
		metrics.GetOrRegisterMeter(m, nil).Mark(int64(msg.meterSize))
		metrics.GetOrRegisterMeter(m+"/packets", nil).Mark(1)
	}
	return nil
}

func (t *rlpxTransport) close(err error) {
	t.wmu.Lock()
	defer t.wmu.Unlock()

	// Disconnect reason をできるだけ送る（plainConn でも write deadline は設定可）
	if t.conn != nil {
		if r, ok := err.(DiscReason); ok && r != DiscNetworkError {
			deadline := time.Now().Add(discWriteTimeout)
			if err := t.conn.SetWriteDeadline(deadline); err == nil {
				t.wbuf.Reset()
				rlp.Encode(&t.wbuf, []DiscReason{r})
				t.conn.Write(discMsg, t.wbuf.Bytes())
			}
		}
		t.conn.Close()
	}
}

// ★ 変更点: パススルー時は ECIES 握手をスキップし、plainConn に差し替え
func (t *rlpxTransport) doEncHandshake(prv *ecdsa.PrivateKey) (*ecdsa.PublicKey, error) {
	if t.passthrough {
		// QUIC(TLS)の上で、RLPx暗号/MACなしのプレーン・フレーミングへ切替
		t.conn = newPlainConn(t.raw)
		return nil, nil // remote pubkey は server.go 側で TLS キャッシュから設定
	}
	// 通常経路（従来どおり）
	t.rlp.SetDeadline(time.Now().Add(handshakeTimeout))
	return t.rlp.Handshake(prv)
}

func (t *rlpxTransport) doProtoHandshake(our *protoHandshake) (their *protoHandshake, err error) {
	// Writing our handshake happens concurrently, we prefer
	// returning the handshake read error. If the remote side
	// disconnects us early with a valid reason, we should return it
	// as the error so it can be tracked elsewhere.
	werr := make(chan error, 1)
	go func() { werr <- Send(t, handshakeMsg, our) }()
	if their, err = readProtocolHandshake(t); err != nil {
		<-werr // make sure the write terminates too
		return nil, err
	}
	if err := <-werr; err != nil {
		return nil, fmt.Errorf("write error: %v", err)
	}
	// If the protocol version supports Snappy encoding, upgrade immediately
	t.conn.SetSnappy(their.Version >= snappyProtocolVersion)

	return their, nil
}

func readProtocolHandshake(rw MsgReader) (*protoHandshake, error) {
	msg, err := rw.ReadMsg()
	if err != nil {
		return nil, err
	}
	if msg.Size > baseProtocolMaxMsgSize {
		return nil, errors.New("message too big")
	}
	if msg.Code == discMsg {
		// Disconnect before protocol handshake is valid according to the
		// spec and we send it ourself if the post-handshake checks fail.
		// We can't return the reason directly, though, because it is echoed
		// back otherwise. Wrap it in a string instead.
		var reason [1]DiscReason
		rlp.Decode(msg.Payload, &reason)
		return nil, reason[0]
	}
	if msg.Code != handshakeMsg {
		return nil, fmt.Errorf("expected handshake, got %x", msg.Code)
	}
	var hs protoHandshake
	if err := msg.Decode(&hs); err != nil {
		return nil, err
	}
	if len(hs.ID) != 64 || !bitutil.TestBytes(hs.ID) {
		return nil, DiscInvalidIdentity
	}
	return &hs, nil
}

// パススルー切替フラグ（既存）
func (t *rlpxTransport) EnablePassthroughMode() { t.passthrough = true }
func (t *rlpxTransport) IsPassthrough() bool    { return t.passthrough }

// ------------------------------------------------------------
// ★ 追加: plainConn 実装（RLPx暗号/MACを外し、簡易ヘッダでフレーミング）
//   ヘッダ: [8B code(uint64 BE)] [4B size(uint32 BE)] + payload
//   Snappy は既存フラグに従って圧縮/展開
// ------------------------------------------------------------
type plainConn struct {
	c      net.Conn
	snappy bool
}

func newPlainConn(c net.Conn) *plainConn { return &plainConn{c: c} }

func (p *plainConn) SetReadDeadline(t time.Time) error  { return p.c.SetReadDeadline(t) }
func (p *plainConn) SetWriteDeadline(t time.Time) error { return p.c.SetWriteDeadline(t) }
func (p *plainConn) SetDeadline(t time.Time) error      { return p.c.SetDeadline(t) }
func (p *plainConn) Close() error                       { return p.c.Close() }
func (p *plainConn) SetSnappy(enable bool)              { p.snappy = enable }

// Read() の wireSize を uint32 に
func (p *plainConn) Read() (code uint64, data []byte, wireSize int, err error) {
	hdr := make([]byte, 12)
	if _, err = io.ReadFull(p.c, hdr); err != nil {
		return 0, nil, 0, err
	}
	code = binary.BigEndian.Uint64(hdr[0:8])
	size := binary.BigEndian.Uint32(hdr[8:12])
	if size > uint32(baseProtocolMaxMsgSize) {
		return 0, nil, 0, errors.New("plain frame too large")
	}
	raw := make([]byte, int(size))
	if _, err = io.ReadFull(p.c, raw); err != nil {
		return 0, nil, 0, err
	}
	wireSize = 12 + len(raw)

	if p.snappy {
		dec, derr := snappy.Decode(nil, raw)
		if derr != nil {
			return 0, nil, 0, derr
		}
		return code, dec, wireSize, nil
	}
	return code, raw, wireSize, nil
}

// Write() の戻り値を uint32 に
func (p *plainConn) Write(code uint64, payload []byte) (uint32, error) {
	out := payload
	if p.snappy {
		out = snappy.Encode(nil, payload)
	}
	if len(out) > baseProtocolMaxMsgSize {
		return 0, errors.New("plain frame too large")
	}
	hdr := make([]byte, 12)
	binary.BigEndian.PutUint64(hdr[0:8], code)
	binary.BigEndian.PutUint32(hdr[8:12], uint32(len(out)))
	if _, err := p.c.Write(hdr); err != nil {
		return 0, err
	}
	if _, err := p.c.Write(out); err != nil {
		return 0, err
	}
	return uint32(len(hdr) + len(out)), nil
}

// QUICパススルー切替用（rlpxTransport が実装）
type tlsBindable interface {
	EnablePassthroughMode()
	IsPassthrough() bool
}
