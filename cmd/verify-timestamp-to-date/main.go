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
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

func main() {
	if err := run(context.Background(), os.Args[1], os.Args[2:]...); err != nil {
		_, _ = os.Stderr.WriteString(`ERROR: ` + err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, command string, args ...string) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	rIn, wIn := io.Pipe()
	defer rIn.Close()
	rOut, wOut := io.Pipe()
	defer rOut.Close()

	inputs := make(chan []byte)
	go func() {
		defer func() { _ = wIn.CloseWithError(context.Cause(ctx)) }()
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				return
			case v := <-inputs:
				if _, err := wIn.Write(v); err != nil {
					cancel(err)
					return
				}
			}
		}
	}()

	outputs := make(chan [2]string)
	go func() {
		defer rOut.Close()
		r := bufio.NewScanner(rOut)
		for r.Scan() {
			i := bytes.IndexByte(r.Bytes(), '\t')
			if i == -1 {
				cancel(errors.New("unexpected output format"))
				return
			}
			select {
			case <-ctx.Done():
				return
			case outputs <- [2]string{string(r.Bytes()[:i]), string(r.Bytes()[i+1:])}:
			}
		}
		if err := r.Err(); err != nil {
			cancel(err)
		}
	}()

	c := exec.CommandContext(ctx, command, args...)
	c.Stderr = os.Stderr
	c.Stdin = rIn
	c.Stdout = wOut

	go func() {
		defer rIn.Close()
		defer wOut.Close()
		var err error
		defer func() {
			cancel(err)
			_ = wOut.CloseWithError(context.Cause(ctx))
		}()
		err = c.Run()
	}()

	var (
		mu  sync.Mutex
		buf []byte
	)

	var convert baseline.TimestampToDate = func(startTime, endTime time.Time) (startDate, endDate string) {
		mu.Lock()
		defer mu.Unlock()

		if ctx.Err() != nil {
			panic(context.Cause(ctx))
		}

		buf = buf[:0]

		if startTime != (time.Time{}) {
			buf = startTime.AppendFormat(buf, time.RFC3339Nano)
		}

		buf = append(buf, '\t')

		if endTime != (time.Time{}) {
			buf = endTime.AppendFormat(buf, time.RFC3339Nano)
		}

		buf = append(buf, '\n')

		select {
		case <-ctx.Done():
			panic(context.Cause(ctx))
		case inputs <- buf:
		}

		select {
		case <-ctx.Done():
			panic(context.Cause(ctx))
		case v := <-outputs:
			return v[0], v[1]
		}
	}

	return baseline.TestTimestampToDateExternal(
		ctx,
		baseline.TimestampRangeValues,
		baseline.DateValues,
		baseline.ExampleMatches,
		convert,
	)
}
