// Package goxbrowser provides a headless browser pool with anti-detection,
// smart fallback and multi-provider support.
//
// Architecture follows the database/sql Strategy Pattern:
//   - Parent package (browser/) defines interfaces and shared capabilities
//   - Sub-packages (rod/, browserless/, surf/) provide concrete implementations
//   - Users import only the providers they need
//
// Available packages:
//   - browser: Core pool, fetcher, detector, worker, metrics
//   - browser/rod: Chrome via go-rod (local + remote)
//   - browser/browserless: Browserless v2 cluster
//   - browser/surf: HTTP TLS fingerprint impersonation
//   - artifact: Storage abstraction (local + S3/OSS)
package goxbrowser
