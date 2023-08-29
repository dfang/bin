// processor.go
//
// process downloaded asset, maybe it is binary file(eg. jq-osx-amd64) or an archive(tar.gz, tar.xz, zip)
package assets

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/apex/log"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/matchers"
	"github.com/h2non/filetype/types"
	"github.com/krolaw/zipstream"
	"github.com/marcosnils/bin/pkg/options"
	bstrings "github.com/marcosnils/bin/pkg/strings"
	"github.com/marcosnils/bin/pkg/util"
	zlog "github.com/rs/zerolog/log"
	"github.com/xi2/xz"
)

// process downloaded asset (tar zip tar.gz or exe)

var (
	msiType = filetype.AddType("msi", "application/octet-stream")
	ascType = filetype.AddType("asc", "text/plain")
)

func (f *Filter) processReader(r io.Reader) (*finalFile, error) {
	zlog.Trace().Msgf("111%v", f.name)

	var buf bytes.Buffer
	tee := io.TeeReader(r, &buf)

	t, err := filetype.MatchReader(tee)
	if err != nil {
		return nil, err
	}

	outputFile := io.MultiReader(&buf, r)

	type processorFunc func(repoName string, r io.Reader) (*finalFile, error)
	var processor processorFunc

	zlog.Debug().Msgf("Processing file %s with matcher %s", f.name, t.Extension)

	switch t {
	case matchers.TypeGz:
		fmt.Println("TypeGz")
		processor = f.processGz
	case matchers.TypeTar:
		fmt.Println("TypeTar")
		processor = f.processTar
	case matchers.TypeXz:
		fmt.Println("TypeXz")
		processor = f.processXz
	case matchers.TypeBz2:
		fmt.Println("TypeBz2")
		processor = f.processBz2
	case matchers.TypeZip:
		fmt.Println("TypeZip")
		processor = f.processZip
	default:
		fmt.Println("file uncompressed")
	}
	// fmt.Printf("filter %+v\n", f)

	if processor != nil {
		outFile, err := processor(f.repoName, outputFile)
		if err != nil {
			return nil, err
		}

		outputFile = outFile.Source

		f.name = outFile.Name
		f.packagePath = outFile.PackagePath

		// In case of e.g. a .tar.gz, process the uncompressed archive by calling recursively
		return f.processReader(outputFile)
	}

	return &finalFile{Source: outputFile, Name: f.name, PackagePath: f.packagePath}, err
}

// nolint: unused
func fileToReader(filePath string) (io.Reader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return file, nil
}

// processGz receives a tar.gz file and returns the
// correct file for bin to download
func (f *Filter) processGz(name string, r io.Reader) (*finalFile, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &finalFile{Source: gr, Name: gr.Name}, nil
}

func (f *Filter) processTar(name string, r io.Reader) (*finalFile, error) {
	tr := tar.NewReader(r)
	tarFiles := map[string][]byte{}
	if len(f.opts.PackagePath) > 0 {
		log.Debugf("Processing tag with PackagePath %s\n", f.opts.PackagePath)
	}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		} else if header.FileInfo().IsDir() {
			continue
		}

		if !f.opts.SkipPathCheck && len(f.opts.PackagePath) > 0 && header.Name != f.opts.PackagePath {
			continue
		}

		if header.Typeflag == tar.TypeReg {
			// TODO we're basically reading all the files
			// isn't there a way just to store the reference
			// where this data is so we don't have to do this or
			// re-scan the archive twice afterwards?
			bs, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			tarFiles[header.Name] = bs
		}
	}
	if len(tarFiles) == 0 {
		return nil, fmt.Errorf("no files found in tar archive, use -p flag to manually select . PackagePath [%s]", f.opts.PackagePath)
	}

	as := make([]*Asset, 0)
	for f := range tarFiles {
		as = append(as, &Asset{Name: f, URL: ""})
	}
	choice, err := f.FilterAssetContents(name, as)
	if err != nil {
		return nil, err
	}
	selectedFile := choice.String()

	tf := tarFiles[selectedFile]

	return &finalFile{Source: bytes.NewReader(tf), Name: filepath.Base(selectedFile), PackagePath: selectedFile}, nil
}

func (f *Filter) processBz2(name string, r io.Reader) (*finalFile, error) {
	br := bzip2.NewReader(r)

	return &finalFile{Source: br, Name: name}, nil
}

func (f *Filter) processXz(name string, r io.Reader) (*finalFile, error) {
	xr, err := xz.NewReader(r, 0)
	if err != nil {
		return nil, err
	}

	return &finalFile{Source: xr, Name: name}, nil
}

func (f *Filter) processZip(name string, r io.Reader) (*finalFile, error) {
	zr := zipstream.NewReader(r)

	zipFiles := map[string][]byte{}
	if len(f.opts.PackagePath) > 0 {
		log.Debugf("Processing tag with PackagePath %s\n", f.opts.PackagePath)
	}
	for {
		header, err := zr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		} else if header.Mode().IsDir() {
			continue
		}

		if !f.opts.SkipPathCheck && len(f.opts.PackagePath) > 0 && header.Name != f.opts.PackagePath {
			continue
		}

		// TODO we're basically reading all the files
		// isn't there a way just to store the reference
		// where this data is so we don't have to do this or
		// re-scan the archive twice afterwards?
		bs, err := io.ReadAll(zr)
		if err != nil {
			return nil, err
		}

		zipFiles[header.Name] = bs
	}
	if len(zipFiles) == 0 {
		return nil, fmt.Errorf("No files found in zip archive. PackagePath [%s]", f.opts.PackagePath)
	}

	as := make([]*Asset, 0)
	for f := range zipFiles {
		as = append(as, &Asset{Name: f, URL: ""})
	}
	choice, err := f.FilterAssetContents(name, as)
	if err != nil {
		return nil, err
	}
	selectedFile := choice.String()

	fr := bytes.NewReader(zipFiles[selectedFile])

	// return base of selected file since tar
	// files usually have folders inside
	return &finalFile{Name: filepath.Base(selectedFile), Source: fr, PackagePath: selectedFile}, nil
}

// isSupportedExt checks if this provider supports
// dealing with this specific file extension
func isSupportedExt(filename string) bool {
	if ext := strings.TrimPrefix(filepath.Ext(filename), "."); len(ext) > 0 {
		switch filetype.GetType(ext) {
		case msiType, matchers.TypeDeb, matchers.TypeRpm, ascType:
			zlog.Trace().Msgf("Filename %s doesn't have a supported extension", filename)
			return false
		case matchers.TypeGz, types.Unknown, matchers.TypeZip, matchers.TypeXz, matchers.TypeTar, matchers.TypeBz2, matchers.TypeExe:
			break
		default:
			zlog.Trace().Msgf("Filename %s doesn't have a supported extension", filename)
			return false
		}
	}

	return true
}

// FilterAssetContents receives a slice of GL assets and tries to
// select the proper one and ask the user to manually select one
// in case it can't determine it
func (f *Filter) FilterAssetContents(repoName string, as []*Asset) (*FilteredAsset, error) {
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
			zlog.Info().Msg("Filter downloaded asset content to determine which is the binary")
			for i := range matches {
				filters := []string{
					"LICENSE",
					"README",
					"CHANGELOG",
					"CREDITS",
					"man",
					"completions",
					"autocomplete",
					"contrib", //
					".1",      // just
				}
				// exclude these extensions
				// exts := []string{
				// 	"1",
				// }
				for _, f := range filters {
					if strings.Contains(matches[i].String(), f) {
						matches[i].score = 0
					}
				}

				// files with ext are non-executable on non-windows systems eg. casey/just
				if util.FileHasExt(matches[i].Name) {
					matches[i].score = matches[i].score - 1
				}

				// eg: jq _jq in gojq
				if util.FileHasExt(matches[i].Name) {
					matches[i].score = matches[i].score + 1
				}

				if matches[i].score > highestAssetScore {
					highestAssetScore = matches[i].score
				}
			}

			for i := len(matches) - 1; i >= 0; i-- {
				if matches[i].score < highestAssetScore {
					zlog.Debug().Msgf("Removing %v with score %v lower than %v", matches[i].Name, matches[i].score, highestAssetScore)
					matches = append(matches[:i], matches[i+1:]...)
				} else {
					zlog.Debug().Msgf("Keeping %v with highest score %v", matches[i].Name, matches[i].score)
				}
			}

		} else {
			zlog.Debug().Msgf("--all flag was supplied, skipping scoring")
			for _, a := range as {
				matches = append(matches, &FilteredAsset{RepoName: repoName, Name: a.Name, DisplayName: a.DisplayName, URL: a.URL, score: 0, Size: a.Size, BrowserDownloadURL: a.BrowserDownloadURL})
			}
		}
	}

	// matches, err := filterBySize(matches)
	// if err != nil {
	// 	return nil, fmt.Errorf("could not find any compatible files")
	// }

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
