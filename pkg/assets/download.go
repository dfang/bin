package assets

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"runtime"
	"time"

	"github.com/cavaliergopher/grab/v3"
	"github.com/cheggaaa/pb"
	zlog "github.com/rs/zerolog/log"
)

// nolint: unused
func extractFilenameFromContentDisposition(contentDisposition string) (string, error) {
	// Use a regular expression to extract the filename from the Content-Disposition header
	re := regexp.MustCompile(`filename="?([^"]+)"?`)
	matches := re.FindStringSubmatch(contentDisposition)
	if len(matches) < 2 {
		return "", fmt.Errorf("filename not found in Content-Disposition header")
	}

	return matches[1], nil
}

// nolint: unused
func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// nolint: unused
func (f *Filter) targetPath(name string) string {
	expectedFilePath := ""
	if runtime.GOOS == "windows" {
		home, homeErr := os.UserHomeDir()
		if homeErr == nil {
			expectedFilePath = path.Join(home, ".cache/", name)
		}
	} else {
		expectedFilePath = path.Join("/tmp/cache", name)
	}
	return expectedFilePath
}

// nolint: unused
func prefixAssetURL(assetURL string) string {
	if proxy := os.Getenv("GITHUB_PROXY_URL"); proxy != "" {
		proxyedAsset, err := url.JoinPath(proxy, assetURL)
		if err != nil {
			return assetURL
		}
		return proxyedAsset
	}
	return assetURL
}

func grabAsset(url string, path string) {
	// create client
	client := grab.NewClient()
	// req, _ := grab.NewRequest(".", url)
	req, _ := grab.NewRequest(path, url)

	// start download
	fmt.Printf("Downloading %v...\n", req.URL())
	resp := client.Do(req)
	fmt.Printf("  %v\n", resp.HTTPResponse.Status)

	writer := io.Discard
	// start new bar
	bar := pb.Full.Start64(resp.Size())
	bar.SetRefreshRate(5 * time.Millisecond)
	file, _ := os.Open(path)

	// create proxy reader
	barReader := bar.NewProxyReader(file)

	// copy from proxy reader
	_, err := io.Copy(writer, barReader)
	if err != nil {
		zlog.Error().Err(err).Msg("error when io.Copy")
	}

	// check for errors
	if err := resp.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
	}

	// finish bar
	bar.Finish()

	fmt.Printf("Download saved to %v \n", resp.Filename)
}

// 1. skip download if file exists
// 2. with timeout, when timeout, try to download with GITHUB_PROXY url
// 3. resuming at break-points
//
// Download asset with timeout and resuming at break-points
// func (f *Filter) Download(gf *FilteredAsset) (*finalFile, error) {
// 	expectedFilePath := f.targetPath(gf)
// 	log.Debugf("expectedFile: %s", expectedFilePath)
// 	filename := filepath.Base(expectedFilePath)

// 	// Check if the file already exists, if yes, get its size
// 	_, err := os.Stat(expectedFilePath)
// 	var startRange int64 = 0
// 	if err == nil {
// 		file, _ := os.Open(expectedFilePath)
// 		startRange, _ = file.Seek(0, io.SeekEnd)
// 		file.Close()
// 	}

// 	f.name = gf.Name

// 	timeout := 5 * time.Second
// 	// Create a context with the specified timeout
// 	ctx, cancel := context.WithTimeout(context.Background(), timeout)
// 	defer cancel()
// 	client := &http.Client{}
// 	// We're not closing the body here since the caller is in charge of that

// 	// prefix AssetURL if env GITHUB_PROXY_URL is set
// 	prefixedAssetURL := prefixAssetURL(gf.URL)
// 	req, err := http.NewRequest(http.MethodGet, prefixedAssetURL, nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	for name, value := range gf.ExtraHeaders {
// 		req.Header.Add(name, value)
// 	}
// 	log.Debugf("Checking binary from %s", prefixedAssetURL)

// 	req = req.WithContext(ctx)
// 	res, err := client.Do(req)
// 	if err != nil {
// 		if ctx.Err() == context.DeadlineExceeded {
// 			fmt.Println("request timed out")
// 			if _, err := os.Stat(expectedFilePath); err == nil {
// 				bar := pb.Full.Start64(0)
// 				expectedFile, _ := os.Open(expectedFilePath)
// 				defer expectedFile.Close()
// 				barReader := bar.NewProxyReader(expectedFile)
// 				defer bar.Finish()
// 				buf := new(bytes.Buffer)
// 				_, err = io.Copy(buf, barReader)
// 				if err != nil {
// 					return nil, err
// 				}
// 				bar.Finish()
// 				return f.processReader(buf)
// 			}

// 		} else {
// 			fmt.Println("Error making request:", err)
// 		}
// 	}
// 	defer res.Body.Close()

// 	if res.StatusCode > 299 || res.StatusCode < 200 {
// 		return nil, fmt.Errorf("%d response when checking binary from %s", res.StatusCode, gf.URL)
// 	}

// 	fmt.Printf("resp: %+v\n", res)
// 	contentDisposition := res.Header.Get("Content-Disposition")
// 	fmt.Printf("contentMd5: %s\n", contentDisposition)
// 	extractedFilename, err := extractFilenameFromContentDisposition(contentDisposition)
// 	if err != nil {
// 		fmt.Println("Error:", err)
// 	}

// 	if _, err = os.Stat(expectedFilePath); err == nil && extractedFilename == filename {
// 		log.Infof("file exists, skip download")
// 		bar := pb.Full.Start64(res.ContentLength)
// 		expectedFile, _ := os.Open(expectedFilePath)
// 		defer expectedFile.Close()
// 		barReader := bar.NewProxyReader(expectedFile)
// 		defer bar.Finish()
// 		buf := new(bytes.Buffer)
// 		_, err = io.Copy(buf, barReader)
// 		if err != nil {
// 			return nil, err
// 		}
// 		bar.Finish()
// 		return f.processReader(buf)
// 	} else {
// 		// filePath := "/tmp/" + gf.Name
// 		body, err := io.ReadAll(res.Body)
// 		if err != nil {
// 			fmt.Println("Error:", err)
// 		}

// 		err = os.WriteFile(expectedFilePath, body, 0777)
// 		if err != nil {
// 			fmt.Println("Error:", err)
// 		}

// 		// We're caching the whole file into memory so we can prompt
// 		// the user which file they want to download

// 		log.Infof("Starting download of %s", gf.URL)
// 		bar := pb.Full.Start64(res.ContentLength)
// 		barReader := bar.NewProxyReader(res.Body)
// 		defer bar.Finish()
// 		buf := new(bytes.Buffer)
// 		_, err = io.Copy(buf, barReader)
// 		if err != nil {
// 			return nil, err
// 		}
// 		bar.Finish()
// 		return f.processReader(buf)
// 	}
// }
