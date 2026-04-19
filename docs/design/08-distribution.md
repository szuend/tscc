# Design Doc 08: Hybrid Distribution

`tscc` is a Go-based tool, but its primary users are TypeScript developers. This document describes a distribution strategy that bridges these two worlds: providing native binaries for hermetic build systems while offering low-friction access via the JS ecosystem's primary package manager, `npm`.

## Goals

- **Universality:** Provide pre-compiled binaries for all major platforms (Linux, macOS, Windows) and architectures (x64, ARM64).
- **Low Friction:** Enable `npm install -g tscc` or `npm install --save-dev tscc` for JS developers.
- **Hermetic Compatibility:** Ensure binaries are easily downloadable as standalone assets for use in Bazel (`http_archive`), Nix, or other build systems without requiring an `npm` registry.
- **Security:** Avoid post-install scripts (e.g., `node-gyp` or `curl`-based installers) which are often blocked in restricted environments or considered a security risk.

## The Strategy

We will use a **hybrid distribution model** inspired by `esbuild`.

### 1. Binary Generation (GoReleaser)

The source of truth for all releases is the Go source code. We use [GoReleaser](https://goreleaser.com/) to automate the build and release process.

- **Artifacts:** For every tag, GoReleaser builds binaries for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, and `windows/amd64`.
- **Packaging:** Each binary is packaged in a `.tar.gz` (or `.zip` for Windows) and uploaded to GitHub Releases.
- **Checksums:** A `checksums.txt` file is generated and signed to ensure artifact integrity.

This satisfies the needs of build systems like Bazel, which prefer pinning to a specific URL and SHA-256 hash.

### 2. npm Distribution (The `esbuild` Pattern)

To support the JS ecosystem, we will publish a suite of packages to `npm`.

#### The Main Package (`tscc`)

The package named `tscc` (or `@szuend/tscc`) is a lightweight wrapper. Its primary responsibilities are:

- **Platform Mapping:** Declaring `optionalDependencies` for every supported platform.
- **Execution Shim:** Providing a small JavaScript entry point (`bin/tscc`) that:
    1.  Determines the current platform and architecture (`process.platform` and `process.arch`).
    2.  Locates the binary within the installed platform-specific package.
    3.  Spawns the binary using `child_process.spawnSync`, passing through all CLI arguments and standard I/O.

#### Platform-Specific Packages

We publish a separate package for each platform/architecture pair (e.g., `@tscc/linux-x64`, `@tscc/darwin-arm64`).

- **Content:** These packages contain *only* the single `tscc` binary and a minimal `package.json`.
- **Filtering:** The `package.json` uses the `os` and `cpu` fields to tell `npm` exactly which environment it belongs to.

**Why this works:** When a user runs `npm install tscc`, npm evaluates the `optionalDependencies`. It will only download and install the *one* package that matches the user's current operating system and CPU architecture. This keeps the installation fast and the `node_modules` footprint small.

## Comparison with Alternatives

| Method | Security | Speed | Reliability | Hermetic-friendly |
| :--- | :--- | :--- | :--- | :--- |
| **`go install`** | High | Slow (compiles) | Medium (requires Go) | No |
| **Post-install `curl`** | Low | Fast | Low (network failure) | No |
| **All-in-one npm** | High | Slow (huge download) | High | No |
| **The `esbuild` Pattern**| **High** | **Fast** | **High** | **Yes** |

## Implementation Plan

1.  **Configure GoReleaser:** Create `.goreleaser.yaml` to handle multi-platform cross-compilation.
2.  **npm Scaffolding:** Create a workspace or script to generate the `package.json` files for the platform packages based on GoReleaser's output.
3.  **The Shim:** Write the `bin/tscc` JavaScript loader.
4.  **CI Integration:** Wire the process into GitHub Actions so that pushing a tag triggers both the GitHub Release and the `npm publish` for all packages.

## Future Considerations

- **Homebrew:** GoReleaser can also manage a Homebrew tap for macOS users who prefer `brew install tscc`.
- **Wasm:** For environments where native binaries are not allowed (e.g., some browser-based playgrounds), we could explore a WebAssembly build of `tscc`, though this would require significant shimming of the file system.
