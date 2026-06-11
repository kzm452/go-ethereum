// p2p/quic_util.go
package p2p

import (
	"net"
	"os"
	"strconv"
	"strings"
)

const envEnableQUIC = "GETH_P2P_QUIC"

// 環境変数でON/OFF制御（まずはCLI改修なしで導入）
func quicEnabled() bool {
	v := os.Getenv(envEnableQUIC)
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "on")
}

// TCPのListenAddrから、QUIC用UDPポート（+1）を導出
// 例: "0.0.0.0:30303" -> "0.0.0.0:30304"
func deriveQUICAddr(tcpListenAddr string) string {
	host, portStr, err := net.SplitHostPort(tcpListenAddr)
	if err != nil {
		// フォールバック
		return ":30403"
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return net.JoinHostPort(host, "30403")
	}
	return net.JoinHostPort(host, strconv.Itoa(p+1))
}
