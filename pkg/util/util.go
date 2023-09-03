package util

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

// bin_0.17.0_Darwin_x86_64 -> bin
// protoc-gen-buf-lint-Darwin-x86_64 -> protoc-gen-buf-lint
// xgit_Windows_x86_64 -> xgit
// CanonicalizeBinaryName remove version, os, arch from filename.
func CanonicalizeBinaryName(filename string) string {
	// Remove versions, operating systems, and architecture identifiers
	patternsToRemove := []string{
		`([-_])([D|d]arwin|[Ll]inux|[Ww]indows|mac[oO][sS]|[Ww]in|apple|osx|freebsd|openbsd|netbsd|solaris|plan9|gnu|musl|unknown)`, // Operating systems
		`([-_])(amd64|arm64|x86_64|x86|x64|i386|armv7|armv6|armv5|s390x|ppc64le)`,                                                   // Architectures
		`([-_])(v?)(\d+(\.\d+)+)`, // Versions like 0.17.0
	}
	sanitized := ""

	for _, pattern := range patternsToRemove {
		re := regexp.MustCompile(pattern)
		filename = re.ReplaceAllString(filename, "")
	}
	// // Remove non-alphanumeric characters and underscores
	// reg := regexp.MustCompile("[^a-zA-Z0-9_.-]")
	// sanitized := reg.ReplaceAllString(filename, "")
	sanitized = filename
	return sanitized
}

// TODO: except .exe ?
func RemoveFileExtension(filename string) string {
	return filename[:len(filename)-len(filepath.Ext(filename))]
}

// files like bat.1, bat.bash definitely non executable.
func FileHasExt(name string) bool {
	// on non-windows systems, executable files are mostly without file extension
	// this can filter out bat.1, bat.bash
	if runtime.GOOS != "windows" {
		if filepath.Ext(name) != "" {
			return true
		}
	}
	return false
}

func IsExecutable(filePath string) bool {
	// Get file information
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}
	// Check if the file is executable
	isExecutable := (fileInfo.Mode() & 0o111) != 0
	if isExecutable {
		fmt.Println("The file is executable.")
	} else {
		fmt.Println("The file is not executable.")
	}
	return true
}
