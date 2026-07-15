package js

import "embed"

// AssetsFS holds the JavaScript runtime assets (the V1/V2 base runtimes and the
// node-style libraries they require). It is the single source of truth for the
// embedded JS runtime and is shared by both the production binary (which passes
// it to InitRuntime) and the package's own tests, so the assets live in exactly
// one place and cannot drift between copies.
//
//go:embed assets/*
var AssetsFS embed.FS
