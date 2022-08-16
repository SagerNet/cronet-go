package main

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sagernet/sing-tools/extensions/log"
	"github.com/sagernet/sing/common"

	"github.com/google/go-github/v44/github"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zip"
	"github.com/ulikunitz/xz"
)

var logger = log.NewLogger("prebuild")

const (
	clangVersion = "llvmorg-15-init-11722-g3f3a235a-2"
)

var (
	goos   string
	goarch string
)

func init() {
	goos = os.Getenv("GOOS")
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch = os.Getenv("GOARCH")
	if goarch == "" {
		goarch = runtime.GOARCH
	}
}

func main() {
	if !common.FileExists("build/llvm/bin/clang") {
		os.RemoveAll("build/llvm")
		os.MkdirAll("build/llvm", 0o755)
		clangDownload := os.ExpandEnv("https://commondatastorage.googleapis.com/chromium-browser-clang/" + clangOsString() + "/clang-" + clangVersion + ".tgz")
		logger.Info(">> ", clangDownload)
		clangDownloadResponse, err := http.Get(clangDownload)
		if err != nil {
			logger.Fatal(err)
		}
		gzReader, err := gzip.NewReader(clangDownloadResponse.Body)
		if err != nil {
			logger.Fatal(err)
		}
		tarReader := tar.NewReader(gzReader)
		linkName := make(map[string]string)
		for {
			header, err := tarReader.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				logger.Fatal(err)
			}
			path := filepath.Join("build/llvm", header.Name)
			if header.FileInfo().IsDir() {
				continue
			}
			logger.Info(">> ", path)
			if header.Linkname != "" {
				linkName[path] = filepath.Join(filepath.Dir(path), header.Linkname)
				linkName[path], _ = filepath.Abs(linkName[path])
				continue
			}
			err = os.MkdirAll(filepath.Dir(path), 0o755)
			if err != nil {
				return
			}
			file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				logger.Fatal(err)
			}

			_, err = io.CopyN(file, tarReader, header.Size)
			if err != nil {
				logger.Fatal(err)
			}
			file.Close()
		}
		clangDownloadResponse.Body.Close()
		var notExists, leftNotExists int
		for {
			for dst, src := range linkName {
				if !common.FileExists(src) {
					notExists++
					continue
				}
				logger.Info(">> ", src, " => ", dst)
				os.MkdirAll(filepath.Dir(dst), 0o755)
				err = os.Symlink(src, dst)
				if err != nil {
					logger.Fatal(err)
				}
				delete(linkName, dst)
			}
			if notExists == 0 {
				break
			}
			if notExists == leftNotExists {
				logger.Fatal("untar: link failed")
			}
			leftNotExists = notExists
			notExists = 0
		}
	}

	output := filepath.Join("build", goos, goarch)
	if !common.FileExists(filepath.Join(output, "libcronet.so")) {
		client := github.NewClient(nil)
		packageRelease, _, err := client.Repositories.GetReleaseByTag(context.Background(), "klzgrad", "naiveproxy", "v104.0.5112.79-2")
		if err != nil {
			logger.Fatal(err)
		}
		if packageRelease == nil {
			logger.Fatal("release not found")
		}
		var packageAsset *github.ReleaseAsset
		hostOs := naiveOsString()
		for _, asset := range packageRelease.Assets {
			if strings.Contains(*asset.Name, hostOs) {
				packageAsset = asset
				break
			}
		}
		if packageAsset == nil {
			logger.Fatal(hostOs, " release not found in ", *packageRelease.AssetsURL)
		}
		logger.Info(">> ", *packageAsset.BrowserDownloadURL)
		response, err := http.Get(*packageAsset.BrowserDownloadURL)
		if err != nil {
			logger.Fatal(err)
		}
		if response.StatusCode != 200 {
			logger.Fatal("HTTP ", response.StatusCode)
		}
		if strings.HasSuffix(*packageAsset.Name, ".tar.xz") {
			reader, err := xz.NewReader(response.Body)
			if err != nil {
				logger.Fatal(err)
			}

			tarReader := tar.NewReader(reader)
			linkName := make(map[string]string)
			for {
				header, err := tarReader.Next()
				if err != nil {
					if err == io.EOF {
						break
					}
					logger.Fatal(err)
				}
				if header.FileInfo().IsDir() {
					continue
				}
				switch _, fileName := filepath.Split(header.Name); fileName {
				default:
					switch filepath.Ext(header.Name) {
					case ".go", ".c", ".sh":
						continue
					}
				}

				path := filepath.Join(output, header.Name[strings.Index(header.Name, "/")+1:])
				logger.Info(">> ", path)
				if header.Linkname != "" {
					linkName[path] = filepath.Join(output, filepath.Dir(header.Name[strings.Index(header.Name, "/")+1:])+"/"+header.Linkname)
					linkName[path], _ = filepath.Abs(linkName[path])
					continue
				}

				err = os.MkdirAll(filepath.Dir(path), 0o755)
				if err != nil {
					return
				}
				file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
				if err != nil {
					logger.Fatal(err)
				}

				_, err = io.CopyN(file, tarReader, header.Size)
				if err != nil {
					logger.Fatal(err)
				}
				file.Close()
			}
			var notExists, leftNotExists int
			for {
				for dst, src := range linkName {
					if !common.FileExists(src) {
						notExists++
						continue
					}
					logger.Info(">> ", src, " => ", dst)
					os.MkdirAll(filepath.Dir(dst), 0o755)
					err = os.Symlink(src, dst)
					if err != nil {
						logger.Fatal(err)
					}
					delete(linkName, dst)
				}
				if notExists == 0 {
					break
				}
				if notExists == leftNotExists {
					logger.Fatal("untar: link failed")
				}
				leftNotExists = notExists
				notExists = 0
			}
		} else {
			content, err := ioutil.ReadAll(response.Body)
			if err != nil {
				logger.Fatal(err)
			}
			zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
			if err != nil {
				logger.Fatal(err)
			}
			for _, file := range zipReader.File {
				if file.FileInfo().IsDir() {
					continue
				}
				name := filepath.Base(file.Name)
				switch filepath.Ext(name) {
				case ".h", ".dll", ".lib":
				default:
					continue
				}
				logger.Info(">> ", name)
				outFile, err := os.Create(name)
				if err != nil {
					logger.Fatal(err)
				}
				input, err := file.OpenRaw()
				if err != nil {
					logger.Fatal(err)
				}
				_, err = io.Copy(outFile, input)
				if err != nil {
					logger.Fatal(err)
				}
				outFile.Close()
			}
		}
		response.Body.Close()
	}
}

func clangOsString() string {
	clangOs := strings.ToUpper(runtime.GOOS[:1]) + runtime.GOOS[1:]
	clangArch := runtime.GOARCH
	switch clangArch {
	case "amd64":
		clangArch = "x64"
	case "386":
		clangArch = "x86"
	case "mipsle":
		clangArch = "mipsel"
	case "mips64le":
		clangArch = "mips64el"
	}
	return clangOs + "_" + clangArch
}

func naiveOsString() string {
	openwrt := os.Getenv("OPENWRT")
	if openwrt != "" {
		return "openwrt-" + openwrt
	}

	switch goos {
	case "windows":
		goos = "win"
	case "darwin":
		goos = "mac"
	}
	switch goarch {
	case "amd64":
		goarch = "x64"
	case "386":
		goarch = "x86"
	case "mipsle":
		goarch = "mipsel"
	case "mips64le":
		goarch = "mips64el"
	}
	return goos + "-" + goarch
}
