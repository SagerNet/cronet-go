//go:build cronet_static

package cronet

// #cgo LDFLAGS: -fuse-ld=lld -Wl,--build-id=sha1 -fPIC -Wl,-z,noexecstack -Wl,-z,relro -Wl,-z,now -Wl,--icf=all -Wl,--color-diagnostics -Wl,--no-call-graph-profile-sort -Wl,--hash-style=sysv --target=mipsel-linux-gnu -mips32 -no-canonical-prefixes -rdynamic -Wl,-z,defs -Wl,--as-needed -pie -Wl,--disable-new-dtags -Wl,--strip-debug -ldl -lpthread -lrt -latomic -lresolv -lm ./build/linux/mipsle/libcronet_static.a
import "C"
