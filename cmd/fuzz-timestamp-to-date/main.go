// Run: go run cmd/verify-timestamp-to-date/main.go ./path/to/your/external/command arg1 arg2 arg3
//
// The external command should read pairs of tab-separated timestamps from
// stdin, and write pairs of tab-separated dates to stdout.
package main

import (
	"context"
	"github.com/joeycumines/dates-timestamps-and-aggregated-data/cmd/fuzz-timestamp-to-date/internal/configuration"
	"github.com/joeycumines/dates-timestamps-and-aggregated-data/internal/quoted"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
)

func main() {
	if err := run(context.Background(), os.Args[1], os.Args[2:]...); err != nil {
		_, _ = os.Stderr.WriteString(`ERROR: ` + err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, command string, args ...string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	_, source, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to find caller source")
	}

	var ldflags string
	{
		var vals []string

		if dir, err := os.Getwd(); err != nil {
			return err
		} else if v, err := configuration.Encode(configuration.Options{
			Cmd:  command,
			Args: args,
			Dir:  dir,
		}); err != nil {
			return err
		} else {
			vals = append(vals, `-X`, configuration.Variable+`=`+v)
		}

		var err error
		ldflags, err = quoted.Join(vals)
		if err != nil {
			return err
		}
	}

	c := exec.CommandContext(
		ctx,
		`go`, `test`,
		`-ldflags=`+ldflags,
		`-fuzz=FuzzTimestampToDate`,
	)
	c.Dir = filepath.Join(filepath.Dir(source), `internal`)

	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin

	ch := make(chan os.Signal, 8)
	signal.Notify(ch)
	defer close(ch)
	defer signal.Stop(ch)

	if err := c.Start(); err != nil {
		return err
	}

	go func() {
		for sig := range ch {
			_ = c.Process.Signal(sig)
		}
	}()

	return c.Wait()
}
