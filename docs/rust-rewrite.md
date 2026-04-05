# Rust Rewrite Notes

## Scope

This repository currently serves one concrete job: cache GitHub repositories as local bare mirrors and expose metadata/statistics around that cache.

The new Rust scaffold in `smart-git-rs/` keeps the same product boundary:

- container-first deployment
- GitHub as the default upstream
- `git-server-http` + `git-server-core` for Smart HTTP protocol handling
- `gix` for repository clone and fetch
- SQLite for cache metadata and counters

## What Was Added

- `smart-git-rs/Cargo.toml`
  - `git-server-http` + `git-server-core` for server-side Git protocol handling
  - `gix` with blocking network client support for mirror sync
  - `rusqlite` with `bundled`
  - `axum` + `tokio` for the HTTP surface
- `smart-git-rs/src/config.rs`
  - container-friendly defaults under `/data/smart-git`
  - `PathBuf` everywhere for path handling
  - configurable TTL and background refresh scan interval
- `smart-git-rs/src/repo_id.rs`
  - validated repo identifiers to avoid path traversal and Windows-reserved names
- `smart-git-rs/src/git/mirror.rs`
  - bare mirror clone/fetch against GitHub using `gix`
- `smart-git-rs/src/db.rs`
  - SQLite schema for mirror metadata and stats
- `smart-git-rs/src/http/admin.rs`
  - `healthz`
  - metadata/stat endpoints
  - a manual sync endpoint to exercise the mirror flow
- `smart-git-rs/src/http/git_http.rs`
  - dynamic `info/refs` and `git-upload-pack` routes
  - TTL-aware mirror sync before protocol handling
  - direct use of the enhanced `git-server-http` endpoint APIs
- `smart-git-rs/src/lifecycle.rs`
  - centralizes repository lifecycle rules, per-repo serialization, SQLite updates, registry registration, TTL gating, and background stale refresh
- `smart-git-rs/Dockerfile`
  - native Docker build path for the Rust service

## Architectural Direction

Using `git-server-http` and `git-server-core` aligns the protocol layer with `gix`, which those crates already use internally. That avoids maintaining two independent Git implementations in the same service.

What `gix` covers well here:

- clone public GitHub repos
- fetch updates into existing bare mirrors
- inspect refs and repository state
- keep repo storage inside the Rust service

What `git-server-http` covers for us:

- `GET /:user/:repo/info/refs?service=git-upload-pack`
- `POST /:user/:repo/git-upload-pack`
- packet-line framing and stateless RPC behavior compatible with Git clients

One implementation detail remains important: `git-server-http::router(store)` assumes repositories are already discovered under a root directory. This project is dynamic, so the current Rust scaffold wraps that router instead of exposing it directly. It performs request-time mirror sync, discovers the relevant repository root, rewrites the incoming URI to the router's static repo layout, and then delegates request handling to the local enhanced `git-server-http` implementation.

Current stage:

1. move cache, metadata, config, and admin APIs to Rust
2. switch mirror management from `git2-rs` to `gix`
3. front the local enhanced `git-server-http` with dynamic registry-backed repository resolution
4. gate upstream fetches with TTL and a background stale refresh scan
5. validate with real Git clients before replacing the Go server

## Why This Shape Improves Portability

- Docker remains the primary deployment target
- `gix` and `git-server-*` stay in the Rust Git ecosystem instead of mixing `libgit2` and native Rust Git implementations
- bundled SQLite avoids system SQLite mismatches
- repo IDs are validated before becoming filesystem paths
- all local paths use `PathBuf` instead of string concatenation

## Suggested Next Steps

1. add integration tests with a real `git clone` client against the Rust service
2. verify response compatibility for `info/refs` and `git-upload-pack` against the current Go implementation
3. port any existing `/api/db/data` and `/api/db/sum` consumers to the Rust process
4. add per-repository refresh de-duplication so concurrent stale requests do not race the same upstream fetch
