package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	"github.com/marcosnils/bin/pkg/config"
	"github.com/marcosnils/bin/pkg/providers"
	"github.com/marcosnils/bin/pkg/util"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type installCmd struct {
	cmd  *cobra.Command
	opts installOpts
}

type installOpts struct {
	force    bool
	provider string
	all      bool
}

func newInstallCmd() *installCmd {
	root := &installCmd{}
	// nolint: dupl
	cmd := &cobra.Command{
		Use:           "install <url>",
		Aliases:       []string{"i"},
		Short:         "Installs the specified project from a url",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			zlog.Trace().Msgf("args: %v", args)
			u := args[0]

			// <OWNER>/<REPO> -> github.com/<OWNER/<REPO>
			if !strings.Contains(u, "github.com") {
				if strings.Contains(u, "/") {
					u = fmt.Sprintf("github.com/%s", u)
				} else {
					u = DEFAULT_SHORTHANDS[u]
				}
			}

			var installDir string
			var fpath, argpath string
			if len(args) > 1 {
				argpath = args[1]
				var err error
				// Resolve to absolute path
				if fpath, err = filepath.Abs(os.ExpandEnv(args[1])); err != nil {
					return err
				}
			} else if len(config.Get().DefaultPath) > 0 {
				fpath = config.Get().DefaultPath
			} else {
				var err error
				fpath, err = os.Getwd()
				if err != nil {
					return err
				}
			}

			// expand ~/ in path
			if strings.HasPrefix(fpath, "~/") {
				dirname, _ := os.UserHomeDir()
				fpath = filepath.Join(dirname, fpath[2:])
			}
			installDir = fpath

			// TODO check if binary already exists in config
			// and triger the update process if that's the case

			p, err := providers.New(u, root.opts.provider)
			if err != nil {
				return err
			}
			zlog.Trace().Msgf("provider %+v", p)

			pResult, err := p.Fetch(&providers.FetchOpts{All: root.opts.all})
			if err != nil {
				return err
			}
			fmt.Printf("pResult %+v\n", pResult)
			fmt.Printf("fpath: %+v\n", fpath)

			fpath, err = checkFinalPath(fpath, pResult.Name)
			if err != nil {
				return err
			}
			fmt.Printf("fpath: %+v\n", fpath)

			if len(argpath) == 0 {
				argpath = fpath
			}
			fmt.Printf("argpath: %+v\n", argpath)

			// fileName := util.CanonicalizeBinaryName(pResult.Name)
			fileName := util.CanonicalizeBinaryName(pResult.Name)
			dpath := path.Join(installDir, fileName)
			fmt.Printf("will install to %s\n", dpath)

			// install binary to path in config
			// fmt.Printf("install binary to path in config: %s\n", "~/bin/")
			fmt.Println("pResult", pResult)
			fmt.Println("dpath", dpath)
			// fmt.Println(root.opts.force)
			if err = installBinary(pResult, dpath, root.opts.force); err != nil {
				return fmt.Errorf("error installing binary: %w", err)
			}

			err = config.UpsertBinary(&config.Binary{
				RemoteName:  pResult.Name,
				Path:        fpath,
				Version:     pResult.Version,
				Hash:        fmt.Sprintf("%x", pResult.Hash.Sum(nil)),
				URL:         u,
				Provider:    p.GetID(),
				PackagePath: pResult.PackagePath,
			})

			if err != nil {
				return err
			}

			zlog.Info().Msgf("Done installing %s %s", pResult.Name, pResult.Version)
			zlog.Info().Msgf("Run %s --help to verify installation", pResult.Name)
			_ = execShell(dpath, []string{"--help"})
			// if err != nil {
			// 	fmt.Println("the installed binary can't not run successfully, please report on github issues ???")
			// }

			return nil
		},
	}

	root.cmd = cmd
	root.cmd.Flags().BoolVarP(&root.opts.force, "force", "f", false, "Force the installation even if the file already exists")
	root.cmd.Flags().BoolVarP(&root.opts.all, "all", "a", false, "Show all possible download options (skip scoring & filtering)")
	root.cmd.Flags().StringVarP(&root.opts.provider, "provider", "p", "", "Forces to use a specific provider")
	return root
}

// checkFinalPath checks if path exists and if it's a dir or not
// and returns the correct final file path. It also
// checks if the path already exists and prompts
// the user to override.
func checkFinalPath(path, fileName string) (string, error) {
	fi, err := os.Stat(os.ExpandEnv(path))

	// TODO implement file existence and override logic
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if fi != nil && fi.IsDir() {
		return filepath.Join(path, fileName), nil
	}

	return path, nil
}

// installBinary saves the specified binary to the desired path
// and makes it executable. It also checks if any other binary
// has the same hash and exists if so.

// TODO check if other binary has the same hash and warn about it.
// TODO if the file is zipped, tared, whatever then extract it.
func installBinary(f *providers.File, path string, overwrite bool) error {
	epath := os.ExpandEnv(path)
	fmt.Println("epath:", epath)

	var extraFlags int = os.O_EXCL

	if overwrite {
		extraFlags = 0
		err := os.Remove(epath)
		log.Debugf("Overwrite flag set, removing file %s\n", epath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	file, err := os.OpenFile(epath, os.O_RDWR|os.O_CREATE|extraFlags, 0o766)
	if err != nil {
		return err
	}

	defer file.Close()

	zlog.Info().Msgf("Copying from %s@%s into %s", f.Name, f.Version, epath)
	_, err = io.Copy(file, f.Data)
	if err != nil {
		return err
	}

	return nil
}

func execShell(command string, args []string) error {
	cmd := exec.Command(command, args...)
	// command.Stdout = os.Stdout
	// command.Stderr = os.Stderr
	// var err = command.Start()
	// if err != nil {
	// 	return err
	// }
	// err = command.Wait()
	// if err != nil {
	// 	return err
	// }

	// Run the command and capture the output and error
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If the command returns an error, handle it here
		fmt.Println("Error:", err)
	}

	// Get the exit code of the command
	exitCode := cmd.ProcessState.ExitCode()

	zlog.Debug().Msgf("Exit Code: %d", exitCode)
	zlog.Debug().Msgf("Output:\n%s\n", output)

	if exitCode != 0 {
		zlog.Info().Msg("the installed binary can't not run successfully, please report on github issues or check issues ???")
	} else {
		zlog.Info().Msg("installation succeed")
	}

	return nil
}

// nolint: unused
func initLogger() {
	output := zerolog.ConsoleWriter{Out: os.Stdout}
	output.TimeFormat = "2006-01-02 15:04:05" // Customize the timestamp format if needed
	// output.FormatLevel = func(i interface{}) string {
	// 	return colorizeLevel(i.(string))
	// }
	zlog.Logger = zlog.Output(output)
}
