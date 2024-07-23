package internal

import (
	"bufio"
	"context"
	"github.com/joeycumines/dates-timestamps-and-aggregated-data/baseline"
	"github.com/joeycumines/dates-timestamps-and-aggregated-data/cmd/fuzz-timestamp-to-date/internal/configuration"
	"github.com/joeycumines/dates-timestamps-and-aggregated-data/cmd/internal/timestamptodate"
	"github.com/joeycumines/dates-timestamps-and-aggregated-data/internal/extcmd"
	"testing"
	"time"
)

func FuzzTimestampToDate(f *testing.F) {
	if configuration.Skip() {
		f.SkipNow()
	}

	options, err := configuration.Decode()
	if err != nil {
		f.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	f.Cleanup(cancel)
	if err := extcmd.Run[[2]time.Time, [2]string](
		ctx,
		f.Helper,
		options.Cmd,
		options.Args,
		options.Dir,
		timestamptodate.AppendInput,
		bufio.ScanLines,
		timestamptodate.ParseOutput,
		func(ctx context.Context, call func(input [2]time.Time) ([2]string, error)) error {
			f.Helper()
			baseline.FuzzTimestampToDate(f, baseline.TimestampRangeValues, baseline.DateValues, timestamptodate.CallToConvert(call))
			return nil
		},
	); err != nil {
		f.Fatal(err)
	}
}
