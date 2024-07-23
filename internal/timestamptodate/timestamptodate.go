package timestamptodate

import (
	"bytes"
	"errors"
	"github.com/joeycumines/dates-timestamps-and-aggregated-data/baseline"
	"time"
)

func AppendInput(b []byte, input [2]time.Time) ([]byte, error) {
	if input[0] != (time.Time{}) {
		b = input[0].AppendFormat(b, time.RFC3339Nano)
	}

	b = append(b, '\t')

	if input[1] != (time.Time{}) {
		b = input[1].AppendFormat(b, time.RFC3339Nano)
	}

	b = append(b, '\n')

	return b, nil
}

func ParseOutput(b []byte) (output [2]string, _ error) {
	i := bytes.IndexRune(b, '\t')
	if i == -1 {
		return output, errors.New("unexpected output format")
	}
	return [2]string{string(b[:i]), string(b[i+1:])}, nil
}

func CallToConvert(call func(input [2]time.Time) ([2]string, error)) baseline.TimestampToDate {
	return func(startTime, endTime time.Time) (startDate, endDate string) {
		v, err := call([2]time.Time{startTime, endTime})
		if err != nil {
			panic(err)
		}
		return v[0], v[1]
	}
}
