package main

func init() {
	cgoCFLAGS("linux", "mips64le", "--target=mips64el-linux-gnuabi64 -march=mips64el -mcpu=mips64r2")
	cgoLDFLAGS("linux", "mips64le", "-fuse-ld=lld --target=mips64el-linux-gnuabi64 -mips64r2")
}
