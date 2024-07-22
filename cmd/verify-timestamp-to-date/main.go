// Run: go run cmd/verify-timestamp-to-date/main.go ./path/to/your/external/command arg1 arg2 arg3
//
// The external command should read pairs of tab-separated timestamps from
// stdin, and write pairs of tab-separated dates to stdout.
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"github.com/joeycumines/dates-timestamps-and-aggregated-data/baseline"
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
		command,
		args,
		func(b []byte, input [2]time.Time) ([]byte, error) {
			if input[0] != (time.Time{}) {
				b = input[0].AppendFormat(b, time.RFC3339Nano)
			}

			b = append(b, '\t')

			if input[1] != (time.Time{}) {
				b = input[1].AppendFormat(b, time.RFC3339Nano)
			}

			b = append(b, '\n')

			return b, nil
		},
		bufio.ScanLines,
		func(b []byte) (output [2]string, _ error) {
			i := bytes.IndexRune(b, '\t')
			if i == -1 {
				return output, errors.New("unexpected output format")
			}
			return [2]string{string(b[:i]), string(b[i+1:])}, nil
		},
		func(ctx context.Context, call func(input [2]time.Time) ([2]string, error)) error {
			return baseline.TestTimestampToDateExternal(
				ctx,
				baseline.TimestampRangeValues,
				baseline.DateValues,
				baseline.ExampleMatches,
				func(startTime, endTime time.Time) (startDate, endDate string) {
					v, err := call([2]time.Time{startTime, endTime})
					if err != nil {
						panic(err)
					}
					return v[0], v[1]
				},
			)
		},
	)
}
