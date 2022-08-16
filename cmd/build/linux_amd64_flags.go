package main

func init() {
	cgoCFLAGS("linux", "amd64", "-m64 -msse3")
	cgoLDFLAGS("linux", "amd64", "-fuse-ld=lld -m64")
}
