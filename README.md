# go-ethereum QUIC Transport Experiment

This repository is an experimental fork of [ethereum/go-ethereum](https://github.com/ethereum/go-ethereum).

It modifies the devp2p/RLPx transport layer of go-ethereum v1.13.15 to experiment with QUIC-based peer-to-peer communication.

This project is intended for research and private network experiments only.

---

## Important Notice

This is **not** an official Ethereum Foundation or go-ethereum release.

Do **not** use this implementation on Ethereum mainnet or in production environments.

The implementation is experimental and has not been audited.

---

## Overview

The original go-ethereum devp2p transport is based on TCP and RLPx.

This fork experimentally adds QUIC support to the P2P transport path. The main changes include:

* QUIC-based outbound dialing
* QUIC-based inbound listener support
* QUIC stream wrapper compatible with Go's `net.Conn` interface
* TLS configuration for QUIC peer authentication
* Environment-variable based QUIC mode switching
* RLPx transport adjustments for QUIC experiments

The implementation is mainly located in:

```text
p2p/dial.go
p2p/server.go
p2p/transport.go
p2p/rlpx/rlpx.go
p2p/quic_conn.go
p2p/quic_listener.go
p2p/quic_tls.go
p2p/quic_util.go
```

---

## QUIC Mode

QUIC mode is controlled by the following environment variable:

```bash
GETH_P2P_QUIC=1
```

Example on Linux/macOS/Git Bash:

```bash
export GETH_P2P_QUIC=1
```

Example on Windows PowerShell:

```powershell
$env:GETH_P2P_QUIC="1"
```

When QUIC mode is enabled, the modified P2P transport attempts to use QUIC for peer communication.

---

## Build

This fork has been tested with:

```text
Go 1.22.0
go-ethereum v1.13.15
Windows amd64
```

To build geth on Windows:

```bash
go build -o build/bin/geth.exe ./cmd/geth
```

To check the version:

```bash
./build/bin/geth.exe version
```

Or:

```bash
go run ./cmd/geth version
```

On Linux/macOS, the output binary name can be changed as needed:

```bash
go build -o build/bin/geth ./cmd/geth
```

---

## Test Status

The geth binary can be built successfully.

Known working command:

```bash
go build ./cmd/geth
```

However, some upstream `p2p` and `p2p/rlpx` tests are not yet fully updated for the modified transport/frame interfaces introduced by this QUIC experiment.

Some tests under the following packages may require additional updates:

```text
p2p
p2p/rlpx
```

This repository should therefore be treated as an experimental research prototype rather than a production-ready implementation.

---

## Purpose

This repository was created for research on replacing or extending Ethereum's peer-to-peer transport layer with QUIC.

The goal is to explore whether QUIC can be used as an alternative transport for devp2p/RLPx communication in a private experimental environment.

---

## Limitations

This implementation is experimental and currently has several limitations:

* Not intended for Ethereum mainnet
* Not production-ready
* Not security-audited
* Compatibility with normal go-ethereum nodes is not guaranteed
* Some upstream tests are not yet updated
* The implementation may require private-network specific configuration

---

## Relationship to go-ethereum

This repository is a fork of the official go-ethereum repository.

Original project:

```text
https://github.com/ethereum/go-ethereum
```

This fork is maintained independently for research purposes and is not affiliated with or endorsed by the Ethereum Foundation or the official go-ethereum maintainers.

---

# 日本語概要

このリポジトリは、[ethereum/go-ethereum](https://github.com/ethereum/go-ethereum) を基にした実験用forkです。

go-ethereum v1.13.15 の devp2p/RLPx トランスポート層を変更し、P2P通信にQUICを利用することを目的とした研究用実装です。

本リポジトリは、研究およびプライベートネットワークでの実験を目的としています。

---

## 注意事項

このリポジトリは、Ethereum Foundation または go-ethereum 公式メンテナによる公式リリースではありません。

Ethereum mainnet や本番環境では使用しないでください。

本実装は実験段階であり、セキュリティ監査は行われていません。

---

## 実装概要

通常のgo-ethereumでは、devp2pの通信にTCPおよびRLPxが用いられています。

本forkでは、P2Pトランスポート層にQUICを導入する実験を行っています。主な変更点は以下の通りです。

* QUICを用いたアウトバウンド接続
* QUICを用いたインバウンドリスナー
* Goの `net.Conn` インターフェースに対応するQUIC stream wrapper
* QUIC peer authentication のためのTLS設定
* 環境変数によるQUICモードの切り替え
* QUIC実験のためのRLPxトランスポート調整

主な変更ファイルは以下です。

```text
p2p/dial.go
p2p/server.go
p2p/transport.go
p2p/rlpx/rlpx.go
p2p/quic_conn.go
p2p/quic_listener.go
p2p/quic_tls.go
p2p/quic_util.go
```

---

## QUICモード

QUICモードは、以下の環境変数で制御します。

```bash
GETH_P2P_QUIC=1
```

Linux/macOS/Git Bash の例:

```bash
export GETH_P2P_QUIC=1
```

Windows PowerShell の例:

```powershell
$env:GETH_P2P_QUIC="1"
```

QUICモードを有効にすると、変更後のP2PトランスポートがQUICを用いた通信を試みます。

---

## ビルド方法

本forkは以下の環境でビルド確認を行っています。

```text
Go 1.22.0
go-ethereum v1.13.15
Windows amd64
```

Windowsでgethをビルドする場合:

```bash
go build -o build/bin/geth.exe ./cmd/geth
```

バージョン確認:

```bash
./build/bin/geth.exe version
```

または:

```bash
go run ./cmd/geth version
```

Linux/macOSでは、必要に応じて出力ファイル名を変更してください。

```bash
go build -o build/bin/geth ./cmd/geth
```

---

## テスト状況

geth本体のビルドは成功しています。

確認済みのコマンド:

```bash
go build ./cmd/geth
```

一方で、QUIC実験に伴いトランスポート層やframe interfaceを変更しているため、既存の `p2p` および `p2p/rlpx` の一部テストはまだ完全には追従していません。

以下のパッケージのテストには追加修正が必要な場合があります。

```text
p2p
p2p/rlpx
```

そのため、本リポジトリは本番利用可能な実装ではなく、研究用プロトタイプとして扱ってください。

---

## 目的

本リポジトリは、EthereumのP2Pトランスポート層をQUICで置き換える、または拡張する可能性を検証するために作成されました。

目的は、プライベートな実験環境において、devp2p/RLPx通信の代替トランスポートとしてQUICを利用できるかを調査することです。

---

## 制限事項

本実装は実験段階であり、以下の制限があります。

* Ethereum mainnetでの利用を想定していない
* 本番環境向けではない
* セキュリティ監査を行っていない
* 通常のgo-ethereumノードとの互換性は保証されない
* 一部の既存テストは未対応
* プライベートネットワーク向けの追加設定が必要になる場合がある

---

## go-ethereumとの関係

このリポジトリは、公式go-ethereumリポジトリのforkです。

元のプロジェクト:

```text
https://github.com/ethereum/go-ethereum
```

本forkは研究目的で独立して管理されており、Ethereum Foundationまたは公式go-ethereumメンテナによって承認・保証されたものではありません。

---

## License

This repository preserves the original license structure of go-ethereum.

The go-ethereum library, which generally includes code outside the `cmd` directory, is licensed under the GNU Lesser General Public License v3.0, as described in `COPYING.LESSER`.

The go-ethereum binaries, which generally include code inside the `cmd` directory, are licensed under the GNU General Public License v3.0, as described in `COPYING`.

Modifications in this fork are intended to follow the same license structure as the original go-ethereum project.

Please see the license files included in this repository for details.
