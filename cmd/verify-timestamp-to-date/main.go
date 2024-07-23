// Run: go run cmd/verify-timestamp-to-date/main.go ./path/to/your/external/command arg1 arg2 arg3
//
// The external command should read pairs of tab-separated timestamps from
// stdin, and write pairs of tab-separated dates to stdout.
package main

import (
	"bufio"
	"context"
	"github.com/joeycumines/dates-timestamps-and-aggregated-data/baseline"
	"github.com/joeycumines/dates-timestamps-and-aggregated-data/cmd/internal/timestamptodate"
	"github.com/joeycumines/dates-timestamps-and-aggregated-data/internal/extcmd"
	"os"
	"time"
)

func main() {
	if err := run(context.Background(), os.Args[1], os.Args[2:]...); err != nil {
		_, _ = os.Stderr.WriteString(`ERROR: ` + err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, command string, args ...string) error {
	return extcmd.Run[[2]time.Time, [2]string](
		ctx,
		nil,
		command,
		args,
		"",
		timestamptodate.AppendInput,
		bufio.ScanLines,
		timestamptodate.ParseOutput,
		func(ctx context.Context, call func(input [2]time.Time) ([2]string, error)) error {
			return baseline.TestTimestampToDateExternal(
				ctx,
				baseline.TimestampRangeValues,
				baseline.DateValues,
				baseline.ExampleMatches,
				timestamptodate.CallToConvert(call),
			)
		},
	)
}
