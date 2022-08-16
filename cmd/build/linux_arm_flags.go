package main

func init() {
	cgoCFLAGS("linux", "arm", "--target=arm-linux-gnueabihf -mfloat-abi=hard -march=armv7-a -mtune=generic-armv7-a -mfpu=neon")
	cgoLDFLAGS("linux", "arm", "-fuse-ld=lld -march=armv7-a --target=arm-linux-gnueabihf -mfloat-abi=hard")
}
