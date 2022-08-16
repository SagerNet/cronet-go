package main

func init() {
	cgoCFLAGS("linux", "386", "-m32 -mfpmath=sse -msse3")
	cgoLDFLAGS("linux", "386", "-fuse-ld=lld -m32")
}
