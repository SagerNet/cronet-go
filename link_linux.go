package cronet

// #cgo LDFLAGS: -Wl,--build-id=sha1 -fPIC -Wl,-z,noexecstack -Wl,-z,relro -Wl,-z,now -Wl,--icf=all -Wl,--color-diagnostics -no-canonical-prefixes -rdynamic -Wl,-z,defs -Wl,--as-needed -Wl,--disable-new-dtags
import "C"
