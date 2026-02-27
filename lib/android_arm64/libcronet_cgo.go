//go:build android && arm64 && !with_purego

package android_arm64

// #cgo LDFLAGS: -L${SRCDIR} -l:libcronet.a -Wl,-wrap,aligned_alloc -Wl,-wrap,calloc -Wl,-wrap,free -Wl,-wrap,malloc -Wl,-wrap,memalign -Wl,-wrap,posix_memalign -Wl,-wrap,pvalloc -Wl,-wrap,realloc -Wl,-wrap,valloc -Wl,-wrap,malloc_usable_size -Wl,-wrap,realpath -Wl,-wrap,strdup -Wl,-wrap,strndup -Wl,-wrap,getcwd -Wl,-wrap,asprintf -Wl,-wrap,vasprintf -ldl -lm -landroid -llog
import "C"

const Version = "143.0.7499.109"
