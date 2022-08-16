//go:build cronet_static

package cronet

// #cgo LDFLAGS: -fuse-ld=lld -Wl,--build-id=sha1 -fPIC -Wl,-z,noexecstack -Wl,-z,relro -Wl,-z,now -Wl,--icf=all -Wl,--color-diagnostics -Wl,-mllvm,-instcombine-lower-dbg-declare=0 -flto=thin -Wl,--thinlto-jobs=all -Wl,--thinlto-cache-dir=thinlto-cache -Wl,--thinlto-cache-policy=cache_size=10%:cache_size_bytes=40g:cache_size_files=100000 -Wl,-mllvm,-import-instr-limit=30 -fwhole-program-vtables -m64 -no-canonical-prefixes -rdynamic -Wl,-z,defs -Wl,--as-needed -Wl,--lto-O2 -pie -Wl,--disable-new-dtags -ldl -lpthread -lrt -latomic -lresolv -lm ./build/linux/amd64/libcronet_static.a
import "C"
