//go:build !cronet_static

package cronet

// #cgo LDFLAGS: -fuse-ld=lld -Wl,--build-id=sha1 -fPIC -Wl,-z,noexecstack -Wl,-z,relro -Wl,-z,now -Wl,--icf=all -Wl,--color-diagnostics -Wl,--no-call-graph-profile-sort -Wl,--hash-style=sysv --target=mips64el-linux-gnuabi64 -mips64r2 -no-canonical-prefixes -rdynamic -Wl,-z,defs -Wl,--as-needed -pie -Wl,--disable-new-dtags -Wl,--strip-debug -ldl -lpthread -lrt ./build/linux/mips64le/libcronet.so -Wl,-rpath,$ORIGIN
import "C"
