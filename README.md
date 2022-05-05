# cronet-go

Note: install cronet libraries and headers from naiveproxy release to current directory or /usr/local

```shell
go build -v -o cronet-example ./example
export LD_LIBRARY_PATH=path to library
./cronet-example https://my-naive-server.com username:password https://example.com
```

Static build:

```shell
CGO_LDFLAGS_ALLOW="-fuse-ld=lld" CC="clang" go build -x -o cronet-example -tags static ./example
./cronet-example https://my-naive-server.com username:password https://example.com
```