package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAsset = "gdu_testos_testarch"

func TestAssetName(t *testing.T) {
	tests := []struct {
		goos, goarch, goarm string
		pie                 bool
		want                string
	}{
		{"linux", "amd64", "", true, "gdu_linux_amd64"},
		{"linux", "amd64", "", false, "gdu_linux_amd64_static"},
		{"linux", "arm64", "", false, "gdu_linux_arm64"},
		{"linux", "arm", "5", false, "gdu_linux_armv5l"},
		{"linux", "arm", "6,softfloat", false, "gdu_linux_armv6l"},
		{"linux", "arm", "7", false, "gdu_linux_armv7l"},
		{"linux", "arm", "", false, "gdu_linux_arm"},
		{"freebsd", "arm", "7", false, "gdu_freebsd_arm"},
		{"darwin", "arm64", "", true, "gdu_darwin_arm64"},
		{"windows", "amd64", "", false, "gdu_windows_amd64.exe"},
		{"android", "arm64", "", false, "gdu_android_arm64"},
	}
	for _, tt := range tests {
		got := assetName(tt.goos, tt.goarch, tt.goarm, tt.pie)
		assert.Equal(t, tt.want, got, "%s/%s goarm=%q pie=%v", tt.goos, tt.goarch, tt.goarm, tt.pie)
	}
}

func TestArchiveName(t *testing.T) {
	assert.Equal(t, "gdu_linux_amd64.tgz", archiveName("gdu_linux_amd64"))
	assert.Equal(t, "gdu_windows_arm64.exe.zip", archiveName("gdu_windows_arm64.exe"))
}

func TestParseVersion(t *testing.T) {
	v, err := parseVersion("v5.39.0")
	assert.NoError(t, err)
	assert.Equal(t, version{5, 39, 0}, v)

	v, err = parseVersion("5.40.12")
	assert.NoError(t, err)
	assert.Equal(t, version{5, 40, 12}, v)

	for _, s := range []string{"development", "", "v5.39.0-3-gabc1234", "v5.39.0-rc.1", "v5.39"} {
		_, err := parseVersion(s)
		assert.Error(t, err, s)
	}
}

func TestVersionNewerThan(t *testing.T) {
	assert.True(t, version{5, 40, 0}.newerThan(version{5, 39, 9}))
	assert.True(t, version{5, 39, 1}.newerThan(version{5, 39, 0}))
	assert.True(t, version{6, 0, 0}.newerThan(version{5, 99, 99}))
	assert.False(t, version{5, 39, 0}.newerThan(version{5, 39, 0}))
	assert.False(t, version{5, 38, 10}.newerThan(version{5, 39, 0}))
}

func TestIsManagedPath(t *testing.T) {
	managed := []string{
		"/opt/homebrew/Cellar/gdu/5.39.0/bin/gdu",
		"/usr/local/Cellar/gdu/5.39.0/bin/gdu",
		"/home/linuxbrew/.linuxbrew/bin/gdu",
		"/snap/gdu-disk-usage-analyzer/123/gdu",
		"/nix/store/abc-gdu-5.39.0/bin/gdu",
		"/home/user/miniconda3/bin/gdu",
		`C:\Users\user\scoop\apps\gdu\current\gdu.exe`,
		`C:\Users\user\AppData\Local\Microsoft\WinGet\Packages\dundee.gdu\gdu.exe`,
		"/home/user/.binenv/binaries/gdu/5.39.0",
		"/home/user/.local/share/mise/installs/gdu/5.39.0/gdu",
		"/home/user/.asdf/installs/gdu/5.39.0/bin/gdu",
	}
	for _, p := range managed {
		assert.True(t, isManagedPath(p), p)
	}

	unmanaged := []string{
		"/usr/bin/gdu",
		"/usr/local/bin/gdu",
		"/home/user/.local/bin/gdu",
		"/home/user/bin/gdu",
		`C:\Tools\gdu.exe`,
	}
	for _, p := range unmanaged {
		assert.False(t, isManagedPath(p), p)
	}
}

func TestExpectedChecksum(t *testing.T) {
	sums := []byte("" +
		"aa  gdu_linux_amd64.tgz\n" +
		"bb *gdu_windows_amd64.exe.zip\n")

	sum, err := expectedChecksum(sums, "gdu_linux_amd64.tgz")
	assert.NoError(t, err)
	assert.Equal(t, []byte{0xaa}, sum)

	sum, err = expectedChecksum(sums, "gdu_windows_amd64.exe.zip")
	assert.NoError(t, err)
	assert.Equal(t, []byte{0xbb}, sum)

	_, err = expectedChecksum(sums, "gdu_darwin_arm64.tgz")
	assert.ErrorContains(t, err, "not listed")
}

func TestExtractZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("gdu_windows_amd64.exe")
	require.NoError(t, err)
	_, err = w.Write([]byte("windows binary"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	bin, err := extract("gdu_windows_amd64.exe.zip", buf.Bytes(), "gdu_windows_amd64.exe")
	assert.NoError(t, err)
	assert.Equal(t, "windows binary", string(bin))

	_, err = extract("gdu_windows_amd64.exe.zip", buf.Bytes(), "gdu_windows_arm64.exe")
	assert.ErrorContains(t, err, "not found")
}

func TestExtractTgz(t *testing.T) {
	bin, err := extract(testAsset+".tgz", makeTgz(t, testAsset, []byte("new binary")), testAsset)
	assert.NoError(t, err)
	assert.Equal(t, "new binary", string(bin))

	_, err = extract(testAsset+".tgz", makeTgz(t, "other", []byte("x")), testAsset)
	assert.ErrorContains(t, err, "not found")
}

func TestUpdateReplacesBinary(t *testing.T) {
	srv := newReleaseServer(t, "v5.40.0", []byte("new binary"), false)
	u := newTestUpdater(t, srv)

	res, err := u.Update(context.Background())

	require.NoError(t, err)
	assert.True(t, res.Updated)
	assert.Equal(t, "v5.39.0", res.CurrentVersion)
	assert.Equal(t, "v5.40.0", res.LatestVersion)
	assertBinary(t, u.ExecPath, "new binary")

	if runtime.GOOS != "windows" {
		info, err := os.Stat(u.ExecPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o750), info.Mode().Perm())
	}
}

func TestUpdateUpToDate(t *testing.T) {
	srv := newReleaseServer(t, "v5.39.0", []byte("new binary"), false)
	u := newTestUpdater(t, srv)

	res, err := u.Update(context.Background())

	require.NoError(t, err)
	assert.False(t, res.Updated)
	assertBinary(t, u.ExecPath, "old binary")
}

func TestUpdateChecksumMismatch(t *testing.T) {
	srv := newReleaseServer(t, "v5.40.0", []byte("new binary"), true)
	u := newTestUpdater(t, srv)

	_, err := u.Update(context.Background())

	assert.ErrorContains(t, err, "checksum")
	assertBinary(t, u.ExecPath, "old binary")
}

func TestUpdateMissingAsset(t *testing.T) {
	srv := newReleaseServer(t, "v5.40.0", []byte("new binary"), false)
	u := newTestUpdater(t, srv)
	u.Asset = "gdu_plan9_amd64"

	_, err := u.Update(context.Background())

	assert.ErrorContains(t, err, "has no asset gdu_plan9_amd64.tgz")
	assertBinary(t, u.ExecPath, "old binary")
}

func TestUpdateRefused(t *testing.T) {
	tests := []struct {
		name   string
		modify func(u *Updater)
		want   string
	}{
		{"disabled", func(u *Updater) { u.Enabled = false }, "cannot update itself"},
		{"development", func(u *Updater) { u.Version = "development" }, "only release builds"},
		{"between tags", func(u *Updater) { u.Version = "v5.39.0-3-gabc1234" }, "only release builds"},
		{"unknown path", func(u *Updater) { u.ExecPath = "" }, "cannot determine"},
		{"package manager", func(u *Updater) { u.ExecPath = "/opt/homebrew/Cellar/gdu/5.39.0/bin/gdu" }, "package manager"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &Updater{
				Client:   http.DefaultClient,
				APIURL:   "http://127.0.0.1:0",
				Version:  "v5.39.0",
				ExecPath: filepath.Join(t.TempDir(), "gdu"),
				Asset:    testAsset,
				Enabled:  true,
			}
			tt.modify(u)

			_, err := u.Update(context.Background())

			assert.ErrorContains(t, err, tt.want)
		})
	}
}

func TestUpdateReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions are not enforced")
	}
	srv := newReleaseServer(t, "v5.40.0", []byte("new binary"), false)
	u := newTestUpdater(t, srv)
	dir := filepath.Dir(u.ExecPath)
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err := u.Update(context.Background())

	assert.ErrorContains(t, err, "sufficient permissions")
	assertBinary(t, u.ExecPath, "old binary")
}

func newTestUpdater(t *testing.T, srv *httptest.Server) *Updater {
	t.Helper()
	execPath := filepath.Join(t.TempDir(), "gdu")
	require.NoError(t, os.WriteFile(execPath, []byte("old binary"), 0o600))
	require.NoError(t, os.Chmod(execPath, 0o750))

	return &Updater{
		Client:   srv.Client(),
		APIURL:   srv.URL + "/repos/dundee/gdu",
		Version:  "v5.39.0",
		ExecPath: execPath,
		Asset:    testAsset,
		Enabled:  true,
	}
}

// newReleaseServer serves a GitHub-like latest release with a single binary.
func newReleaseServer(t *testing.T, tag string, bin []byte, badChecksum bool) *httptest.Server {
	t.Helper()
	archive := makeTgz(t, testAsset, bin)
	sum := sha256.Sum256(archive)
	if badChecksum {
		sum[0] ^= 0xff
	}
	sums := fmt.Sprintf("%s  %s.tgz\n", hex.EncodeToString(sum[:]), testAsset)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/repos/dundee/gdu/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		rel := release{
			TagName: tag,
			Assets: []releaseAsset{
				{Name: testAsset + ".tgz", URL: srv.URL + "/download/" + testAsset + ".tgz"},
				{Name: sumsAsset, URL: srv.URL + "/download/" + sumsAsset},
			},
		}
		_ = json.NewEncoder(w).Encode(rel)
	})
	mux.HandleFunc("/download/"+testAsset+".tgz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/download/"+sumsAsset, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sums))
	})
	return srv
}

func makeTgz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     name,
		Mode:     0o755,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}))
	_, err := tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func assertBinary(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}
