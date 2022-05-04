# cronet-go

Note: download libcronet.so from naiveproxy release to /usr/lib/local or any shared link path

Example:

```shell
go build -v -o cronet-example ./example
# go install -v github.com/sagernet/sing/cli/libpack@v0.0.0-20220504232156-cc559556fc61
# export PATH="$PATH:$(go env GOPATH)/bin"
libpack -i cronet-example
./cronet-example https://my-naive-server.com username:password https://example.com
```