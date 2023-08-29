package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	zlog "github.com/rs/zerolog/log"
)

var setupDone bool

func TestMain(m *testing.M) {
	fmt.Println("setup")

	// This code will run before any tests are executed
	setup()

	// Run the actual tests
	result := m.Run()

	// You can add any teardown or cleanup logic here

	// Exit with the result of the tests
	os.Exit(result)
}

func TestIntegration(t *testing.T) {
	if !setupDone {
		t.Fatal("Setup not done before running the test")
	}

	t.Parallel()

	// Set up test data
	tt := []struct {
		Name     string
		Repo     string
		ExitCode int
	}{
		{
			Name:     "assh",
			Repo:     "github.com/moul/assh",
			ExitCode: 0,
		},
		{
			Name:     "fzf",
			Repo:     "github.com/junegunn/fzf",
			ExitCode: 0,
		},
	}

	// Call main()
	// Assert expected results

	for _, test := range tt {
		fn := func(t *testing.T) {
			if e, a := test.ExitCode, run_cmd(test.Repo, []string{""}); e != a {
				t.Errorf("expected exit code %d for %s, got exit code %d", e, test.Repo, a)
			}
		}

		t.Run(test.Name, fn)
	}
}

func setup() {
	cmd := exec.Command("go", "build", "-o", "/tmp/bin")
	_, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
	}
	configData := `{ "default_path": "~/bin/"}`
	// Get the home directory path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting home directory:", err)
		return
	}

	// Construct the full path to the config directory
	configDir := filepath.Join(homeDir, ".config", "bin")

	// Create directories recursively if they don't exist
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		fmt.Println("Error creating directories:", err)
		return
	}

	// Construct the full path to the config file
	configPath := filepath.Join(configDir, "config.json")

	// Write the configuration data to the file
	err = os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		return
	}
	fmt.Println("Configuration data written to config.json")

	// Your setup code here
	// This will be executed before any tests
	setupDone = true
}

func run_cmd(name string, args []string) int {
	install_args := []string{"i", "-f", name, "/tmp/"}
	cmd := exec.Command("/tmp/bin", install_args...)
	zlog.Debug().Msgf("cmd:\n%s\n", cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
	}
	zlog.Debug().Msgf("Output:\n%s\n", output)
	exitCode := cmd.ProcessState.ExitCode()
	return exitCode
}
