# cronet-go

Go bindings for [naiveproxy](https://github.com/klzgrad/naiveproxy).

## Build cronet locally

```bash
git submodule update --init --recursive
go run ./cmd/build-naive build
go run ./cmd/build-naive package
```

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
| linux/amd64   | linux   | x64   |
| linux/arm64   | linux   | arm64 |
| windows/amd64 | win     | x64   |
| windows/arm64 | win     | arm64 |
