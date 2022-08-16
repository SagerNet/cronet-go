package main

func init() {
	cgoCFLAGS("linux", "arm64", "-mbranch-protection=pac-ret --target=aarch64-linux-gnu")
	cgoLDFLAGS("linux", "arm64", "-fuse-ld=lld --target=aarch64-linux-gnu")
}
