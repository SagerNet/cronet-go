package main

func init() {
	cgoCFLAGS("linux", "mipsle", "--target=mipsel-linux-gnu -march=mipsel -mcpu=mips32 -mhard-float")
	cgoLDFLAGS("linux", "mipsle", "-fuse-ld=lld --target=mipsel-linux-gnu -mips32")
}
