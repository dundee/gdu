package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"runtime"
	"runtime/debug"
	"strings"
)

const maxBinarySize = 500 << 20

// defaultAsset returns the name of the release binary matching the running one.
func defaultAsset() string {
	goarm, pie := "", false
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			switch s.Key {
			case "GOARM":
				goarm = s.Value
			case "-buildmode":
				pie = s.Value == "pie"
			}
		}
	}
	return assetName(runtime.GOOS, runtime.GOARCH, goarm, pie)
}

// assetName mirrors the binary names produced by `make build-all`.
func assetName(goos, goarch, goarm string, pie bool) string {
	name := "gdu_" + goos + "_" + goarch
	switch {
	case goos == "windows":
		return name + ".exe"
	case goos == "linux" && goarch == "arm" && goarm != "":
		// GOARM may carry a float ABI suffix, e.g. "6,softfloat"
		return "gdu_linux_armv" + goarm[:1] + "l"
	case goos == "linux" && goarch == "amd64" && !pie:
		return name + "_static"
	}
	return name
}

func archiveName(asset string) string {
	if strings.HasSuffix(asset, ".exe") {
		return asset + ".zip"
	}
	return asset + ".tgz"
}

// extract returns the contents of the file called member inside the archive.
func extract(archive string, data []byte, member string) ([]byte, error) {
	var (
		bin []byte
		err error
	)
	if strings.HasSuffix(archive, ".zip") {
		bin, err = extractZip(data, member)
	} else {
		bin, err = extractTgz(data, member)
	}
	if err != nil {
		return nil, fmt.Errorf("extracting %s from %s: %w", member, archive, err)
	}
	return bin, nil
}

func extractTgz(data []byte, member string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("file not found in archive")
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeReg && path.Base(hdr.Name) == member {
			return readLimited(tr)
		}
	}
}

func extractZip(data []byte, member string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	for _, f := range zr.File {
		if f.Mode().IsRegular() && path.Base(f.Name) == member {
			return readZipFile(f)
		}
	}
	return nil, errors.New("file not found in archive")
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return readLimited(rc)
}

func readLimited(r io.Reader) ([]byte, error) {
	bin, err := io.ReadAll(io.LimitReader(r, maxBinarySize+1))
	if err != nil {
		return nil, err
	}
	if len(bin) > maxBinarySize {
		return nil, errors.New("binary is unexpectedly large")
	}
	return bin, nil
}
