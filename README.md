# ESD

[![Tag](https://img.shields.io/github/tag/attestantio/esd.svg)](https://github.com/attestantio/esd/releases/)
[![License](https://img.shields.io/github/license/attestantio/esd.svg)](LICENSE)
[![GoDoc](https://godoc.org/github.com/attestantio/esd?status.svg)](https://godoc.org/github.com/attestantio/esd)
![Lint](https://github.com/attestantio/esd/workflows/golangci-lint/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/attestantio/esd)](https://goreportcard.com/report/github.com/attestantio/esd)

`esd` is a process that watches for slashing events included on the Ethereum beacon chain and runs a script when found.

## Table of Contents

- [Install](#install)
  - [Binaries](#binaries)
  - [Docker](#docker)
  - [Source](#source)
- [Usage](#usage)
- [Maintainers](#maintainers)
- [Contribute](#contribute)
- [License](#license)

## Install

### Binaries

Binaries for the latest version of `esd` can be obtained from [the releases page](https://github.com/attestantio/esd/releases/latest).

### Docker

You can obtain the latest version of `esd` using docker with:

```
docker pull attestantio/esd
```

### Source

`esd` is a standard Go binary which can be installed with:

```sh
go get github.com/attestantio/esd
```

# Usage

# Requirements to run `esd`
## Beacon node
`esd` supports all beacon nodes that support the beacon API.

# Configuring `esd`
The minimal requirements for `esd` are references to the beacon node, for example:

```
esd --eth2client.address=localhost:5051
```

Here, 'eth2client.address' is the address of a supported beacon client node (gRPC for Prysm, HTTP for Teku and Lighthouse).

To be useful, `esd` should be supplied with the names of scripts to run when slashings are detected.  A configuration file containing this is shown below:

```yaml
eth2client:
  address: 'localhost:5051'
slashings:
  attester-slashed-script: '/home/esd/scripts/attester-slashed.sh'
  proposer-slashed-script: '/home/esd/scripts/proposer-slashed.sh'
```

These scripts are called when attester and proposer slashings are found on the beacon chain.  The scripts are passed a single argument, which is the index of the validator for which the slashing has been obtained.

## Maintainers

Jim McDonald: [@mcdee](https://github.com/mcdee).

## Contribute

Contributions welcome. Please check out [the issues](https://github.com/attestantio/esd/issues).

## License

[Apache-2.0](LICENSE) © 2021 - 2023 Attestant Limited.
