package assets

import (
	"fmt"
	"runtime"
	"sort"
	"strings"

	"github.com/apex/log"
	"github.com/marcosnils/bin/pkg/options"
	bstrings "github.com/marcosnils/bin/pkg/strings"
	zlog "github.com/rs/zerolog/log"
)

// filter assets from a release to determine which asset to download
// filter asset contents when extracting archive

// design consideration:
// order by scoring by os, arch, file extension, file transfer size

// choose air_1.44.0_linux_amd64.tar.gz over air_1.44.0_linux_amd64 because of less network transfer
// air_1.44.0_windows_arm64.tar.gz over air_1.44.0_windows_arm64.exe
// rtx-v1.35.8-macos-arm64.tar.xz > rtx-v1.35.8-macos-arm64.tar.gz > rtx-v1.35.8-macos-arm64 (transfer size)
// rtx-v1.35.8-macos-arm64.tar.gz > rtx-v1.35.8-macos-arm64.tar.xz > rtx-v1.35.8-macos-arm64 (accorging to chatgpt, Both tar and gzip are commonly available on most Linux distributions as they are fundamental tools for working with compressed archives. However, the availability of xz might vary.)

// to make it simple, filter the assets with same scores with less file size
// 1. first round, match os, arch, file extension
// 2. second round, choose the smallest file size
// user should have tar gzip xz or unzip installed

type Filter struct {
	opts        *FilterOpts
	repoName    string
	name        string
	packagePath string
}

type FilterOpts struct {
	SkipScoring   bool
	SkipPathCheck bool

	// If target file is in a package format (tar, zip,etc) use this
	// variable to filter the resulting outputs. This is very useful
	// so we don't prompt the user to pick the file again on updates
	PackagePath string
}

func InitFilter(repoName string, name string, packagePath string, opts *FilterOpts) *Filter {
	return &Filter{
		repoName:    repoName,
		name:        name,
		packagePath: packagePath,
		opts:        opts,
	}
}

func NewFilter(opts *FilterOpts) *Filter {
	return &Filter{opts: opts}
}

func (g FilteredAsset) String() string {
	if g.DisplayName != "" {
		return g.DisplayName
	}
	return g.Name
}

// FilterAssets receives a slice of GL assets and tries to
// select the proper one and ask the user to manually select one
// in case it can't determine it
func (f *Filter) FilterAssets(repoName string, as []*Asset) (*FilteredAsset, error) {
	matches := []*FilteredAsset{}
	if len(as) == 1 {
		a := as[0]
		matches = append(matches, &FilteredAsset{RepoName: repoName, Name: a.Name, URL: a.URL, score: 0, Size: a.Size, BrowserDownloadURL: a.BrowserDownloadURL})
	} else {
		if !f.opts.SkipScoring {
			scores := map[string]int{}
			scoreKeys := []string{}
			scores[repoName] = 1
			for _, os := range resolver.GetOS() {
				scores[os] = 10
			}
			for _, arch := range resolver.GetArch() {
				scores[arch] = 5
			}
			for _, osSpecificExtension := range resolver.GetOSSpecificExtensions() {
				scores[osSpecificExtension] = 15
			}

			for key := range scores {
				scoreKeys = append(scoreKeys, strings.ToLower(key))
			}

			zlog.Debug().Str("GOOS", runtime.GOOS).Str("GOARCH", runtime.GOARCH).Msg("Guess the proper asset to be downloaded based on the target system info")

			for _, a := range as {
				highestScoreForAsset := 0
				gf := &FilteredAsset{RepoName: repoName, Name: a.Name, DisplayName: a.DisplayName, URL: a.URL, score: 0, Size: a.Size, BrowserDownloadURL: a.BrowserDownloadURL}
				for _, candidate := range []string{a.Name} {
					candidateScore := 0
					if bstrings.ContainsAny(strings.ToLower(candidate), scoreKeys) &&
						isSupportedExt(candidate) {
						for toMatch, score := range scores {
							if strings.Contains(strings.ToLower(candidate), strings.ToLower(toMatch)) {
								candidateScore += score
							}
						}
						if candidateScore > highestScoreForAsset {
							highestScoreForAsset = candidateScore
							gf.Name = candidate
							gf.score = candidateScore
						}
					}
				}

				if highestScoreForAsset > 0 {
					matches = append(matches, gf)
				}
			}
			highestAssetScore := 0
			for i := range matches {
				// // github.com/sharkdp/bat
				// filters := []string{
				// 	"LICENSE",
				// 	"README",
				// 	"autocomplete",
				// 	"CHANGELOG",
				// }
				// for _, f := range filters {
				// 	if strings.Contains(matches[i].String(), f) {
				// 		matches[i].score = 0
				// 	}
				// }
				if matches[i].score > highestAssetScore {
					highestAssetScore = matches[i].score
				}
			}

			for i := len(matches) - 1; i >= 0; i-- {
				if matches[i].score < highestAssetScore {
					// log.Debugf("Removing %v (URL %v) with score %v lower than %v", matches[i].Name, matches[i].BrowserDownloadURL, matches[i].score, highestAssetScore)
					zlog.Trace().Msgf("Removing %v with score %v lower than %v", matches[i].BrowserDownloadURL, matches[i].score, highestAssetScore)
					matches = append(matches[:i], matches[i+1:]...)
				} else {
					// log.Debugf("Keeping %v (URL %v) with highest score %v", matches[i].Name, matches[i].BrowserDownloadURL, matches[i].score)
					zlog.Trace().Msgf("Keeping %v with highest score %v", matches[i].BrowserDownloadURL, matches[i].score)
				}
			}

		} else {
			log.Debugf("--all flag was supplied, skipping scoring")
			for _, a := range as {
				matches = append(matches, &FilteredAsset{RepoName: repoName, Name: a.Name, DisplayName: a.DisplayName, URL: a.URL, score: 0, Size: a.Size, BrowserDownloadURL: a.BrowserDownloadURL})
			}
		}
	}

	// ok for jdxcode/rtx
	// not for cloudflare/cfssl
	// matches, err := filterBySize(matches)
	// if err != nil {
	// 	return nil, fmt.Errorf("could not find any compatible files")
	// }
	zlog.Debug().Msg("After filtering")
	for i := 0; i < len(matches); i++ {
		zlog.Debug().Str("URL", matches[i].BrowserDownloadURL).Int("score", matches[i].score).Msg("Filtered candidates:")
	}

	var gf *FilteredAsset
	if len(matches) == 0 {
		return nil, fmt.Errorf("could not find any compatible files")
	} else if len(matches) > 1 {
		generic := make([]fmt.Stringer, 0)
		for _, f := range matches {
			generic = append(generic, f)
		}

		sort.SliceStable(generic, func(i, j int) bool {
			return generic[i].String() < generic[j].String()
		})

		choice, err := options.Select("Multiple matches found, please select one:", generic)
		if err != nil {
			return nil, err
		}
		gf = choice.(*FilteredAsset)
		// TODO make user select the proper file
	} else {
		gf = matches[0]
	}

	return gf, nil
}

// nolint: unused
// filterBySize: keep the smallest size one for assets with same scores, eg. jdxcode/rtx
func filterBySize(assets []*FilteredAsset) ([]*FilteredAsset, error) {
	if len(assets) == 0 {
		return []*FilteredAsset{}, fmt.Errorf("could not find any compatible files") // Return an empty file if the slice is empty
	}

	smallest := assets[0]
	for _, asset := range assets {
		if asset.Size < smallest.Size {
			smallest = asset
		}
	}
	matches := []*FilteredAsset{}
	matches = append(matches, smallest)
	return matches, nil
}
