// p2p/quic_tls.go
package p2p

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"math/big"
	"sync"
	"time"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

var (
	// テスト用私用OID（必要なら自社PENに差し替え）
	oidTLSBind = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 55555, 1, 1}
	// key: hex(sha256(leaf.Raw)) -> *ecdsa.PublicKey (secp256k1)
	verifiedNodeIDCache sync.Map
)

const tlsBindContext = "geth-quic-tlsbind-v1"

type tlsBindExt struct {
	Secp256k1Pub []byte `asn1:"octet"` // 65B uncompressed
	SigRS        []byte `asn1:"octet"` // 64B r||s (no recovery id)
}

func cacheKeyFromLeaf(leaf *x509.Certificate) string {
	sum := sha256.Sum256(leaf.Raw)
	return hex.EncodeToString(sum[:])
}

func LoadAndRemoveVerifiedNodeID(key string) (*ecdsa.PublicKey, bool) {
	v, ok := verifiedNodeIDCache.LoadAndDelete(key)
	if !ok {
		return nil, false
	}
	pub, ok := v.(*ecdsa.PublicKey)
	return pub, ok
}

func makeQUICServerTLSConfig(nodeKey *ecdsa.PrivateKey) (*tls.Config, error) {
	cfg, err := makeQUICCommonTLSConfig(nodeKey, true)
	if err != nil {
		return nil, err
	}
	// ★ クライアント証明書を必須にする（これで ClientHello 後に必ず証明書が送られる）
	cfg.ClientAuth = tls.RequireAnyClientCert
	// 念のため明示（共通側でも TLS1.3 を有効化しているがダブっても問題なし）
	cfg.MinVersion = tls.VersionTLS13
	cfg.NextProtos = []string{"geth-rlpx-plain/1"}
	return cfg, nil
}

// p2p/quic_tls.go

func makeQUICClientTLSConfig(nodeKey *ecdsa.PrivateKey) (*tls.Config, error) {
	cfg, err := makeQUICCommonTLSConfig(nodeKey, false)
	if err != nil {
		return nil, err
	}
	// サーバーとALPNを一致させる (必須)
	cfg.NextProtos = []string{"geth-rlpx-plain/1"}
	cfg.MinVersion = tls.VersionTLS13
	return cfg, nil
}

func makeQUICCommonTLSConfig(nodeKey *ecdsa.PrivateKey, _ bool) (*tls.Config, error) {
	// 1) TLS用一時鍵（P-256）
	tlsKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	tpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		NotBefore:             time.Now().Add(-1 * time.Minute),
		NotAfter:              time.Now().Add(30 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// 2) 拡張：secp256k1 公開鍵 + 署名(SPKI||context)
	spki, err := x509.MarshalPKIXPublicKey(&tlsKey.PublicKey)
	if err != nil {
		return nil, err
	}
	h := gethcrypto.Keccak256(append(spki, []byte(tlsBindContext)...))
	sig65, err := gethcrypto.Sign(h, nodeKey) // 65B (r,s,v)
	if err != nil {
		return nil, err
	}
	sig64 := sig65[:64]

	secpPub := gethcrypto.FromECDSAPub(&nodeKey.PublicKey) // 65B
	extBytes, _ := asn1.Marshal(tlsBindExt{Secp256k1Pub: secpPub, SigRS: sig64})
	tpl.ExtraExtensions = []pkix.Extension{{Id: oidTLSBind, Critical: false, Value: extBytes}}

	// 3) 自己署名
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &tlsKey.PublicKey, tlsKey)
	if err != nil {
		return nil, err
	}
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: tlsKey}

	// 4) VerifyPeerCertificate で検証→キャッシュ保存
	// p2p/quic_tls.go の verify 関数

	verify := func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			log.Error("QUIC tlsbind: no peer cert") // ★ 追加
			return errors.New("no peer cert")
		}
		leaf, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			log.Error("QUIC tlsbind: cert parse failed", "err", err) // ★ 追加
			return err
		}
		// 拡張取り出し
		var ext tlsBindExt
		found := false
		for _, e := range leaf.Extensions {
			if e.Id.Equal(oidTLSBind) {
				if _, err := asn1.Unmarshal(e.Value, &ext); err != nil {
					log.Error("QUIC tlsbind: asn1 unmarshal failed", "err", err) // ★ 追加
					return err
				}
				found = true
				break
			}
		}
		if !found {
			log.Error("QUIC tlsbind: ext missing") // ★ 追加
			return errors.New("tlsbind ext missing")
		}
		// 署名検証
		spki := leaf.RawSubjectPublicKeyInfo
		h := gethcrypto.Keccak256(append(spki, []byte(tlsBindContext)...))
		if len(ext.SigRS) != 64 {
			log.Error("QUIC tlsbind: bad sig len") // ★ 追加
			return errors.New("bad sig len")
		}
		if !gethcrypto.VerifySignature(ext.Secp256k1Pub, h, ext.SigRS) {
			log.Error("QUIC tlsbind: sig verify FAILED") // ★ 追加
			return errors.New("tlsbind sig verify failed")
		}
		pub, err := gethcrypto.UnmarshalPubkey(ext.Secp256k1Pub)
		if err != nil {
			log.Error("QUIC tlsbind: unmarshal pubkey failed", "err", err) // ★ 追加
			return err
		}
		key := cacheKeyFromLeaf(leaf)
		verifiedNodeIDCache.Store(key, pub)
		time.AfterFunc(2*time.Minute, func() { verifiedNodeIDCache.Delete(key) })

		log.Trace("QUIC tlsbind: verification SUCCESS", "key", key) // ★ トレースログ追加
		return nil
	}

	return &tls.Config{
		Certificates:          []tls.Certificate{cert},
		InsecureSkipVerify:    true, // trust is enforced by VerifyPeerCertificate
		VerifyPeerCertificate: verify,
		MinVersion:            tls.VersionTLS13,
	}, nil
}
