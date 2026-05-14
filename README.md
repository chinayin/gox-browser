# gox-browser

[![Go Reference](https://pkg.go.dev/badge/github.com/chinayin/gox-browser.svg)](https://pkg.go.dev/github.com/chinayin/gox-browser)
[![Go Report Card](https://goreportcard.com/badge/github.com/chinayin/gox-browser)](https://goreportcard.com/report/github.com/chinayin/gox-browser)
[![License](https://img.shields.io/github/license/chinayin/gox-browser)](LICENSE)

Headless browser pool with anti-detection, smart fallback and multi-provider support. 浏览器池：反检测、智能降级、多 Provider 统一抽象。

## Packages

- **[browser](browser/)** - Core browser pool, fetcher, WAF detector, worker, metrics
  - **[browser/rod](browser/rod/)** - Chrome provider via go-rod (local + remote)
  - **[browser/browserless](browser/browserless/)** - Browserless v2 cluster provider
  - **[browser/surf](browser/surf/)** - HTTP TLS fingerprint impersonation provider
- **[artifact](artifact/)** - Storage abstraction (local filesystem + S3/OSS)

## Installation

```bash
go get github.com/chinayin/gox-browser
```

## Requirements

- Go 1.25 or higher

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/chinayin/gox-browser/browser"
    "github.com/chinayin/gox-browser/browser/rod"
    "github.com/chinayin/gox-browser/browser/surf"
)

func main() {
    pool := browser.NewPool(browser.PoolConfig{
        MaxInstances:    4,
        IdleTimeout:     2 * time.Minute,
        AcquireTimeout:  30 * time.Second,
        HealthCheckFreq: 30 * time.Second,
    })
    defer pool.Close()

    pool.Register(rod.NewProvider(rod.Config{
        Headless:          true,
        StealthMode:       true,
        RandomFingerprint: true,
    }))
    pool.Register(surf.NewProvider(surf.Config{}))

    detector := browser.NewBlockDetector()
    fetcher, _ := browser.NewFetcher(pool, detector,
        browser.WithPrimary(browser.TypeSurf),
        browser.WithFallback(browser.TypeRodHeadless),
    )

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    result, err := fetcher.Fetch(ctx, "https://example.com")
    if err != nil {
        panic(err)
    }

    fmt.Printf("Title: %s\n", result.Title)
    fmt.Printf("Type:  %s\n", result.FinalType.String())
    fmt.Printf("HTML:  %d bytes\n", len(result.HTML))
}
```

## Features

- **Multi-provider pool** - Rod (Chrome), Browserless (remote cluster), Surf (HTTP TLS)
- **Smart fallback** - Automatic degradation chain when blocked by WAF
- **Anti-detection** - Stealth mode, fingerprint randomization, evasion scripts
- **WAF detection** - Cloudflare, Akamai, generic WAF/captcha detection
- **Concurrent worker** - Batch URL fetching with rate limiting
- **Metrics** - Pool stats, latency percentiles (P50/P95/P99)
- **Artifact storage** - Local filesystem + S3/OSS for screenshots and results

## Development

```bash
# Run tests
make test

# Run linter
make lint

# Run all checks
make check
```

## Documentation

Full documentation is available at [pkg.go.dev](https://pkg.go.dev/github.com/chinayin/gox-browser).

## License

[Apache-2.0](LICENSE)
