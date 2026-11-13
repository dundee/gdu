// Package selfupdate replaces the running gdu binary with the latest release
// published on GitHub.
//
// Only binaries attached to GitHub releases can update themselves. Builds made
// by package managers are left to the package manager, see
// docs/adr/0004-self-update-only-for-release-binaries.md.
package selfupdate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fynelabs/selfupdate"

	"github.com/dundee/gdu/v5/build"
)

// DefaultAPIURL is the GitHub API endpoint of the gdu repository.
const DefaultAPIURL = "https://api.github.com/repos/dundee/gdu"

const (
	sumsAsset       = "sha256sums.txt"
	maxSumsSize     = 1 << 20
	maxArchiveSize  = 200 << 20
	maxReleaseSize  = 10 << 20
	githubMediaType = "application/vnd.github+json"
)

// Result describes the outcome of a successful Update call.
type Result struct {
	CurrentVersion string
	LatestVersion  string
	// Updated is false when the running binary is already the latest release.
	Updated bool
}

// Updater replaces the binary at ExecPath with the latest release.
type Updater struct {
	Client *http.Client
	// APIURL is the GitHub API URL of the repository releases are read from.
	APIURL string
	// Version is the version of the running binary, e.g. v5.39.0.
	Version string
	// ExecPath is the resolved path of the binary to replace.
	ExecPath string
	// Asset is the name of the binary inside the release archive,
	// e.g. gdu_linux_amd64. The archive is the same name plus .tgz or .zip.
	Asset string
	// Enabled is set only in binaries built for GitHub releases.
	Enabled bool
}

// New returns an Updater for the running binary.
func New() *Updater {
	execPath, err := os.Executable()
	if err == nil {
		execPath, err = filepath.EvalSymlinks(execPath)
	}
	if err != nil {
		execPath = ""
	}

	return &Updater{
		Client:   http.DefaultClient,
		APIURL:   DefaultAPIURL,
		Version:  build.Version,
		ExecPath: execPath,
		Asset:    defaultAsset(),
		Enabled:  build.SelfUpdate == "true",
	}
}

// Update downloads the latest release and replaces the binary at ExecPath
// with it, unless the running binary is already up to date.
func (u *Updater) Update(ctx context.Context) (Result, error) {
	res := Result{CurrentVersion: u.Version}

	current, err := u.checkAllowed()
	if err != nil {
		return res, err
	}

	rel, err := u.latestRelease(ctx)
	if err != nil {
		return res, err
	}
	latest, err := parseVersion(rel.TagName)
	if err != nil {
		return res, fmt.Errorf("latest release %q: %w", rel.TagName, err)
	}
	res.LatestVersion = rel.TagName
	if !latest.newerThan(current) {
		return res, nil
	}

	bin, err := u.download(ctx, rel)
	if err != nil {
		return res, err
	}
	if err := u.replace(bin); err != nil {
		return res, err
	}

	res.Updated = true
	return res, nil
}

func (u *Updater) checkAllowed() (version, error) {
	if !u.Enabled {
		return version{}, errors.New(
			"this build of gdu cannot update itself, update it the same way you installed it (e.g. with your package manager)",
		)
	}

	current, err := parseVersion(u.Version)
	if err != nil {
		return version{}, fmt.Errorf("only release builds can update themselves, this is version %q", u.Version)
	}

	if u.ExecPath == "" {
		return version{}, errors.New("cannot determine the location of the gdu binary")
	}
	if isManagedPath(u.ExecPath) {
		return version{}, fmt.Errorf(
			"%s looks like it was installed by a package manager, update it with the package manager instead", u.ExecPath,
		)
	}
	if err := checkWritable(filepath.Dir(u.ExecPath)); err != nil {
		return version{}, fmt.Errorf("cannot replace %s (%w), re-run with sufficient permissions (e.g. sudo)", u.ExecPath, err)
	}

	return current, nil
}

type release struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

func (r *release) assetURL(name string) (string, error) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL, nil
		}
	}
	return "", fmt.Errorf("release %s has no asset %s", r.TagName, name)
}

func (u *Updater) latestRelease(ctx context.Context) (*release, error) {
	data, err := u.get(ctx, u.APIURL+"/releases/latest", githubMediaType, maxReleaseSize)
	if err != nil {
		return nil, fmt.Errorf("checking the latest release: %w", err)
	}

	var rel release
	if err := json.Unmarshal(data, &rel); err != nil {
		return nil, fmt.Errorf("decoding the latest release: %w", err)
	}
	return &rel, nil
}

// download fetches the release archive, verifies its checksum and returns the
// binary inside it.
func (u *Updater) download(ctx context.Context, rel *release) ([]byte, error) {
	archive := archiveName(u.Asset)

	archiveURL, err := rel.assetURL(archive)
	if err != nil {
		return nil, err
	}
	sumsURL, err := rel.assetURL(sumsAsset)
	if err != nil {
		return nil, err
	}

	sums, err := u.get(ctx, sumsURL, "", maxSumsSize)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", sumsAsset, err)
	}
	expected, err := expectedChecksum(sums, archive)
	if err != nil {
		return nil, err
	}

	data, err := u.get(ctx, archiveURL, "", maxArchiveSize)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", archive, err)
	}
	if actual := sha256.Sum256(data); !bytes.Equal(actual[:], expected) {
		return nil, fmt.Errorf("checksum of %s does not match %s", archive, sumsAsset)
	}

	return extract(archive, data, u.Asset)
}

func (u *Updater) replace(bin []byte) error {
	mode := os.FileMode(0o755)
	if info, err := os.Stat(u.ExecPath); err == nil {
		mode = info.Mode().Perm()
	}

	err := selfupdate.Apply(bytes.NewReader(bin), selfupdate.Options{
		TargetPath: u.ExecPath,
		TargetMode: mode,
	})
	if err == nil {
		return nil
	}
	if rerr := selfupdate.RollbackError(err); rerr != nil {
		return fmt.Errorf(
			"replacing %s failed (%w) and restoring the old binary failed too (%s), reinstall gdu manually",
			u.ExecPath, err, rerr.Error(),
		)
	}
	return fmt.Errorf("replacing %s: %w", u.ExecPath, err)
}

func (u *Updater) get(ctx context.Context, url, accept string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gdu/"+u.Version)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}

	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("GET %s: response larger than %d bytes", url, limit)
	}
	return data, nil
}

// expectedChecksum finds the checksum of name in the output of sha256sum.
func expectedChecksum(sums []byte, name string) ([]byte, error) {
	for line := range strings.SplitSeq(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			sum, err := hex.DecodeString(fields[0])
			if err != nil {
				return nil, fmt.Errorf("invalid checksum of %s in %s: %w", name, sumsAsset, err)
			}
			return sum, nil
		}
	}
	return nil, fmt.Errorf("%s is not listed in %s", name, sumsAsset)
}

func checkWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".gdu-update-*")
	if err != nil {
		return err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return err
	}
	return os.Remove(name)
}
