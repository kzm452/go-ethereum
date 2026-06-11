// p2p/quic_conn.go
package p2p

import (
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

// quicStreamConn は「1つのQUICコネクション上の1本のストリーム」を
// net.Conn っぽく見せるラッパーです。
type quicStreamConn struct {
	conn  quic.Connection
	str   quic.Stream
	laddr net.Addr
	raddr net.Addr

	verifiedKey string
}

func newQuicStreamConn(c quic.Connection, s quic.Stream, verifiedKey string) net.Conn {
	return &quicStreamConn{
		conn:        c,
		str:         s,
		laddr:       c.LocalAddr(),
		raddr:       c.RemoteAddr(),
		verifiedKey: verifiedKey,
	}
}

func (q *quicStreamConn) PeerCertSerial() (string, bool) {
	if q.verifiedKey == "" {
		return "", false
	}
	return q.verifiedKey, true
}

func (q *quicStreamConn) Read(p []byte) (int, error)  { return q.str.Read(p) }
func (q *quicStreamConn) Write(p []byte) (int, error) { return q.str.Write(p) }

func (q *quicStreamConn) Close() error {
	// ストリームを閉じてから、コネクションをエラー無しで閉じる
	_ = q.str.Close()
	return q.conn.CloseWithError(0, "")
}

func (q *quicStreamConn) LocalAddr() net.Addr  { return q.laddr }
func (q *quicStreamConn) RemoteAddr() net.Addr { return q.raddr }

func (q *quicStreamConn) SetDeadline(t time.Time) error {
	_ = q.str.SetDeadline(t)
	return nil
}
func (q *quicStreamConn) SetReadDeadline(t time.Time) error {
	_ = q.str.SetReadDeadline(t)
	return nil
}
func (q *quicStreamConn) SetWriteDeadline(t time.Time) error {
	_ = q.str.SetWriteDeadline(t)
	return nil
}

func (q *quicStreamConn) GetVerifiedNodeIDKey() string { return q.verifiedKey }
