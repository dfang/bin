package main

import (
	"fmt"
	"os/exec"
	"testing"

	zlog "github.com/rs/zerolog/log"
)

func TestIntegration(t *testing.T) {
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
				t.Errorf("expected exit code %d, got exit code %d", e, a)
			}
		}

		t.Run(test.Name, fn)
	}
}

func run_cmd(name string, args []string) int {
	install_args := []string{"i", "-f", name, "/tmp/"}
	cmd := exec.Command("/tmp/bin", install_args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
	}
	zlog.Debug().Msgf("Output:\n%s\n", output)
	exitCode := cmd.ProcessState.ExitCode()
	return exitCode
}
