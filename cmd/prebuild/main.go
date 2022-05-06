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

	"github.com/google/go-github/v44/github"
	"github.com/klauspost/compress/zip"
	"github.com/sagernet/sing/common/log"
	"github.com/ulikunitz/xz"
)

var logger = log.NewLogger("prebuild")

func main() {
	client := github.NewClient(nil)
	releases, _, err := client.Repositories.ListReleases(context.Background(), "klzgrad", "naiveproxy", nil)
	if err != nil {
		logger.Fatal(err)
	}
	var packageRelease *github.RepositoryRelease
	for _, release := range releases {
		name := *release.Name
		if name == "" {
			name = *release.TagName
		}
		if strings.HasPrefix(name, "cronet-") {
			packageRelease = release
			break
		}
	}
	if packageRelease == nil {
		logger.Fatal("cronet release not found")
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
			name := filepath.Base(header.Name)
			switch filepath.Ext(name) {
			case ".h", ".so", ".dylib", ".a":
			default:
				continue
			}
			logger.Info(">> ", name)
			file, err := os.Create(name)
			if err != nil {
				logger.Fatal(err)
			}
			_, err = io.CopyN(file, tarReader, header.Size)
			if err != nil {
				logger.Fatal(err)
			}
			file.Close()
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

func naiveOsString() string {
	goos := os.Getenv("GOOS")
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := os.Getenv("GOARCH")
	if goarch == "" {
		goarch = runtime.GOARCH
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
