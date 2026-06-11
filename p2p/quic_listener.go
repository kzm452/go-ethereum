// p2p/quic_listener.go
package p2p

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"

	"github.com/quic-go/quic-go"
)

type quicNetListener struct{ l *quic.Listener }

func newQuicNetListener(l *quic.Listener) net.Listener { return &quicNetListener{l: l} }

func (l *quicNetListener) Accept() (net.Conn, error) {
	sess, err := l.l.Accept(context.Background())
	if err != nil {
		return nil, err
	}
	str, err := sess.AcceptStream(context.Background())
	if err != nil {
		_ = sess.CloseWithError(0, "accept stream failed")
		return nil, err
	}
	// TLS state から leaf を取り出し fingerprint を生成
	cs := sess.ConnectionState().TLS
	key := ""
	if len(cs.PeerCertificates) > 0 {
		leaf := cs.PeerCertificates[0]
		sum := sha256.Sum256(leaf.Raw)
		key = hex.EncodeToString(sum[:])
	}
	return newQuicStreamConn(sess, str, key), nil
}

func (l *quicNetListener) Close() error   { return l.l.Close() }
func (l *quicNetListener) Addr() net.Addr { return l.l.Addr() }
