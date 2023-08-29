package util

import (
	"testing"
)

func TestCanonicalizeBinaryName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"bin_0.17.0_Darwin_x86_64", "bin"},
		{"bin_v0.17.0_Darwin_x86_64", "bin"},
		{"protoc-gen-buf-lint-Darwin-x86_64", "protoc-gen-buf-lint"},
		{"xgit_Windows_x86_64", "xgit"},
		{"fzf-0.42.0-darwin_amd64", "fzf"},
		{"zoxide-0.9.2-x86_64-apple-darwin", "zoxide"},
		{"rtx-v1.35.8-macos-x64", "rtx"},
		{"jq-osx", "jq"},
		{"choose-x86_64-unknown-linux-gnu", "choose"},
		{"choose-unknown-linux-musl", "choose"},
		{"fzf-0.42.0-windows_armv7", "fzf"},
		{"fzf-0.42.0-windows_armv6", "fzf"},
		{"fzf-0.42.0-windows_armv5", "fzf"},
		{"fzf-0.42.0-linux_s390x", "fzf"},
		{"fzf-0.42.0-linux_ppc64le", "fzf"},
		{"rclone-v1.63.1-freebsd-amd64", "rclone"},
		{"rclone-v1.63.1-netbsd-amd64", "rclone"},
		{"rclone-v1.63.1-openbsd-amd64", "rclone"},
		{"rclone-v1.63.1-plan9-amd64", "rclone"},
		{"rclone-v1.63.1-solaris-amd64", "rclone"},
		// Add more test cases as needed
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := CanonicalizeBinaryName(test.input)
			if result != test.expected {
				t.Errorf("Given: %s, Expected: %s, Got: %s", test.input, test.expected, result)
			}
		})
	}
}
