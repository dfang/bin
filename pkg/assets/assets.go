package assets

import (
	"bytes"
	"io"
	"os"
	"path"
	"strings"

	"github.com/cheggaaa/pb"
	"github.com/marcosnils/bin/pkg/config"
	zlog "github.com/rs/zerolog/log"
)

type Asset struct {
	Name string
	// Some providers (like gitlab) have non-descriptive names for files,
	// so we're using this DisplayName as a helper to produce prettier
	// outputs for bin
	DisplayName        string
	URL                string
	Size               int64
	BrowserDownloadURL string
}

func (g Asset) String() string {
	if g.DisplayName != "" {
		return g.DisplayName
	}
	return g.Name
}

// TODO: use BrowserDownloadURL to download not API URL
// see ReleaseAsset struct in go-github/github/repo_releases.go
type FilteredAsset struct {
	RepoName           string
	Name               string
	DisplayName        string
	URL                string // API URL: https://api.github.com/repos/BurntSushi/ripgrep/releases/assets/38486907
	BrowserDownloadURL string // BrowserDownloadURL: https://github.com/junegunn/fzf/releases/download/0.42.0/fzf-0.42.0-darwin_amd64.zip
	score              int
	Size               int64
	ContentMd5         string
	ExtraHeaders       map[string]string
}

type finalFile struct {
	Source      io.Reader
	Name        string
	PackagePath string
}

// SanitizeName removes irrelevant information from the
// file name in case it exists
func SanitizeName(name, version string) string {
	name = strings.ToLower(name)
	replacements := []string{}

	// TODO maybe instead of doing this put everything in a map (set) and then
	// generate the replacements? IDK.
	firstPass := true
	for _, osName := range resolver.GetOS() {
		for _, archName := range resolver.GetArch() {
			replacements = append(replacements, "_"+osName+archName, "")
			replacements = append(replacements, "-"+osName+archName, "")
			replacements = append(replacements, "."+osName+archName, "")

			if firstPass {
				replacements = append(replacements, "_"+archName, "")
				replacements = append(replacements, "-"+archName, "")
				replacements = append(replacements, "."+archName, "")
			}
		}

		replacements = append(replacements, "_"+osName, "")
		replacements = append(replacements, "-"+osName, "")
		replacements = append(replacements, "."+osName, "")

		firstPass = false
	}

	replacements = append(replacements, "_"+version, "")
	replacements = append(replacements, "_"+strings.TrimPrefix(version, "v"), "")
	replacements = append(replacements, "-"+version, "")
	replacements = append(replacements, "-"+strings.TrimPrefix(version, "v"), "")
	r := strings.NewReplacer(replacements...)
	return r.Replace(name)
}

// ProcessURL processes a FilteredAsset by uncompressing/unarchiving the URL of the asset.
func (f *Filter) ProcessURL(gf *FilteredAsset) (*finalFile, error) {
	zlog.Debug().Msgf("cache_dir: %s", config.GetCacheDir())
	expectedFilePath := path.Join(config.GetCacheDir(), gf.Name)
	zlog.Debug().Msgf("expectedFilePath: %s", expectedFilePath)
	// filename := filepath.Base(expectedFilePath)

	grabAsset(gf.BrowserDownloadURL, expectedFilePath)
	f.name = gf.Name

	zlog.Info().Msg("processing downloaded asset ...")
	bar := pb.Full.Start64(0)
	expectedFile, _ := os.Open(expectedFilePath)
	defer expectedFile.Close()
	barReader := bar.NewProxyReader(expectedFile)
	defer bar.Finish()
	buf := new(bytes.Buffer)
	_, err := io.Copy(buf, barReader)
	if err != nil {
		return nil, err
	}
	bar.Finish()
	return f.processReader(buf)
}
