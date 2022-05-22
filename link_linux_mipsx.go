//go:build mipsle || mips64le

package cronet

// #cgo LDFLAGS: -Wl,--no-call-graph-profile-sort -Wl,--hash-style=sysv
import "C"
