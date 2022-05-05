# cronet-go

Note: download libcronet.so/libcronet_static.a from naiveproxy release to current directory or /usr/local/lib


```shell
go build -v -o cronet-example ./example
# go install -v github.com/sagernet/sing/cli/libpack@v0.0.0-20220504232156-cc559556fc61
# export PATH="$PATH:$(go env GOPATH)/bin"
libpack -i cronet-example
./cronet-example https://my-naive-server.com username:password https://example.com
```

Static build:

```shell
CGO_LDFLAGS_ALLOW="-fuse-ld=lld" CC="clang" go build -x -o cronet-example -tags static ./example
```