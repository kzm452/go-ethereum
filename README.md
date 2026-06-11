# go-ethereum QUIC Transport Experiment

This repository is an experimental fork of [ethereum/go-ethereum](https://github.com/ethereum/go-ethereum).

It modifies the devp2p/RLPx transport layer of go-ethereum v1.13.15 to experiment with QUIC-based peer-to-peer communication.

This project is intended for research and private network experiments only.

## Important Notice

This is **not** an official Ethereum Foundation or go-ethereum release.

Do **not** use this implementation on Ethereum mainnet or in production environments.
The implementation is experimental and has not been audited.

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

## QUIC Mode

QUIC mode is controlled by the following environment variable:

```bash
GETH_P2P_QUIC=1
```

Example:

```bash
export GETH_P2P_QUIC=1
```

On Windows PowerShell:

```powershell
$env:GETH_P2P_QUIC="1"
```

When QUIC mode is enabled, the modified P2P transport attempts to use QUIC for peer communication.

## Build

This fork has been tested with:

```text
Go 1.22.0
go-ethereum v1.13.15
Windows amd64
```

To build geth:

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

## Test Status

The geth binary can be built successfully.

However, some upstream `p2p` and `p2p/rlpx` tests are not yet fully updated for the modified transport/frame interfaces introduced by this QUIC experiment.

Known status:

```text
go build ./cmd/geth
```

works.

Some tests under:

```text
p2p
p2p/rlpx
```

may require additional updates.

## Purpose

This repository was created for research on replacing or extending Ethereum's peer-to-peer transport layer with QUIC.

The goal is to explore whether QUIC can be used as an alternative transport for devp2p/RLPx communication in a private experimental environment.

## Limitations

This implementation is experimental and currently has several limitations:

* Not intended for Ethereum mainnet
* Not production-ready
* Not security-audited
* Compatibility with normal go-ethereum nodes is not guaranteed
* Some upstream tests are not yet updated
* The implementation may require private-network specific configuration

## Relationship to go-ethereum

This repository is a fork of the official go-ethereum repository.

Original project:

```text
https://github.com/ethereum/go-ethereum
```

This fork is maintained independently for research purposes and is not affiliated with or endorsed by the Ethereum Foundation or the official go-ethereum maintainers.

## License

This repository preserves the original license structure of go-ethereum.

The go-ethereum library, which generally includes code outside the `cmd` directory, is licensed under the GNU Lesser General Public License v3.0, as described in `COPYING.LESSER`.

The go-ethereum binaries, which generally include code inside the `cmd` directory, are licensed under the GNU General Public License v3.0, as described in `COPYING`.

Modifications in this fork are intended to follow the same license structure as the original go-ethereum project.

Please see the license files included in this repository for details.