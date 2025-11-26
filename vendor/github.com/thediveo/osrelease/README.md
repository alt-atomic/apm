<!-- markdownlint-disable-next-line MD022 -->
# A Pedantic `os-release` Parser

[![License](https://img.shields.io/github/license/thediveo/osrelease)](https://img.shields.io/github/license/thediveo/osrelease)
[![GitHub](https://img.shields.io/github/license/thediveo/osrelease)](https://img.shields.io/github/license/thediveo/osrelease)
![build and test](https://github.com/thediveo/osrelease/actions/workflows/buildandtest.yaml/badge.svg?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/thediveo/osrelease)](https://goreportcard.com/report/github.com/thediveo/osrelease)
![Coverage](https://img.shields.io/badge/Coverage-100.0%25-brightgreen)

`osrelease` is a Go module implementing a slightly pedantic parser for the
os-release file format, as specified in
[os-release(5)](https://www.freedesktop.org/software/systemd/man/os-release.html).
This implementation actually _parses_ quoted assignment values for escape
sequences in _correct sequence_, instead of simply doing blind
search-and-replaces (which can result in unexpected results).

Oh, and this module has _tests_.

There's a reason why we call it the "_pedantic_" parser after all.

For devcontainer instructions, please see the [section "DevContainer"
below](#devcontainer).

## Usage

Simply get the OS identification variables using `osrelease.New()`. That's all.

```go
package main

import "github.com/thediveo/osrelease"

func main() {
    vars := osrelease.New()
    for name, value := range vars {
        println(name, "=", value)
    }
}
```

## DevContainer

> [!CAUTION]
>
> Do **not** use VSCode's "~~Dev Containers: Clone Repository in Container
> Volume~~" command, as it is utterly broken by design, ignoring
> `.devcontainer/devcontainer.json`.

1. `git clone https://github.com/thediveo/enumflag`
2. in VSCode: Ctrl+Shift+P, "Dev Containers: Open Workspace in Container..."
3. select `enumflag.code-workspace` and off you go...

## Supported Go Versions

`native` supports versions of Go that are noted by the [Go release
policy](https://golang.org/doc/devel/release.html#policy), that is, major
versions _N_ and _N_-1 (where _N_ is the current major version).

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md).

## Copyright and License

`osrelease` is Copyright 2021, 2025 Harald Albrecht, and licensed under the
Apache License, Version 2.0.
