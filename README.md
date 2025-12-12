# cronet-go

[![Reference](https://pkg.go.dev/badge/github.com/sagernet/cronet-go.svg)](https://pkg.go.dev/github.com/sagernet/cronet-go)

Go bindings for [naiveproxy](https://github.com/klzgrad/naiveproxy).

## Supported Platforms

| Target        | OS      | CPU   |
|---------------|---------|-------|
| android/386   | android | x86   |
| android/amd64 | android | x64   |
| android/arm   | android | arm   |
| android/arm64 | android | arm64 |
| darwin/amd64  | mac     | x64   |
| darwin/arm64  | mac     | arm64 |
| ios/arm64     | ios     | arm64 |
| linux/386     | linux   | x86   |
| linux/amd64   | linux   | x64   |
| linux/arm     | linux   | arm   |
| linux/arm64   | linux   | arm64 |
| windows/386   | win     | x86   |
| windows/amd64 | win     | x64   |
| windows/arm64 | win     | arm64 |

## System Requirements

| Platform      | Minimum Version |
|---------------|-----------------|
| macOS         | 12.0 (Monterey) |
| iOS           | 15.0            |
| Windows       | 10              |
| Android       | 5.0 (API 21)    |
| Linux (glibc) | glibc 2.31      |
| Linux (musl)  | any             |

## Downstream Build Requirements

| Platform      | Requirements       | Go Build Flags                    |
|---------------|--------------------|-----------------------------------|
| Linux (glibc) | Chromium toolchain | -                                 |
| Linux (musl)  | Chromium toolchain | `-tags with_musl`                 |
| macOS / iOS   | macOS Xcode        | -                                 |
| Windows       | -                  | `CGO_ENABLED=0 -tags with_purego` |
| Android       | Android NDK        | -                                 |

## Linux Build instructions

```bash
git clone --recursive --depth=1 https://github.com/sagernet/cronet-go.git
cd cronet-go
go run ./cmd/build-naive --target=linux/amd64 download-toolchain
#go run ./cmd/build-naive --target=linux/amd64 --libc=musl download-toolchain

# Outputs CC, CXX, and CGO_LDFLAGS=-fuse-ld=lld
export $(go run ./cmd/build-naive --target=linux/amd64 env)
#export $(go run ./cmd/build-naive --target=linux/amd64 --libc=musl env)

cd /path/to/your/project
go build
# go build -tags with_musl
```

### Directories to cache

```yaml
- cronet-go/naiveproxy/src/third_party/llvm-build/Release+Asserts/
- cronet-go/naiveproxy/src/out/sysroot-build/
```
