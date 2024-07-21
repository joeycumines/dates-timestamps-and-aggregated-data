// Package baseline provides an implementation for verifying the logic
// described in the project README.md, and is unit tested and fuzz tested.
package baseline

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

const (
	// DateFormat is used for parsing and formatting dates (inc. test data).
	DateFormat = `2006-01-02`

	// TimestampFormat is used for parsing and formatting timestamps (inc. test data).
	TimestampFormat = "2006-01-02T15:04:05.999999999Z07:00" // RFC 3339 (ns)

	oneDay = 24 * time.Hour
)

type (
	// TimestampToDate is intended for comparison against a date in UTC.
	// N.B. The input range [startTime, endTime) maps to the output range
	// [startDate, endDate] (endTime is exclusive, all other values are
	// inclusive).
	// Default values are treated as not set / ignored.
	TimestampToDate func(startTime, endTime time.Time) (startDate, endDate string)

	// DateToTimestamp is intended to select data by timestamp, between a date range.
	// N.B. The input range [startDate, endDate] maps to the output range
	// [startTime, endTime) (endTime is exclusive, all other values are
	// inclusive).
	// Default values are treated as not set / ignored.
	DateToTimestamp func(startDate, endDate string) (startTime, endTime time.Time)
)

// MatchesTimestamp demonstrates matching a timestamp against a range.
// The endTime is exclusive because time is continuous. That said, in
// practice, most implementations have somewhere between 1 second and 1
// nanosecond resolution.
func MatchesTimestamp(startTime, endTime, value time.Time) bool {
	if startTime != (time.Time{}) && value.Before(startTime) {
		return false
	}
	if endTime != (time.Time{}) && !value.Before(endTime) { // exclusive
		return false
	}
	return true
}

// MatchesDate demonstrates matching a date against a range.
// Unlike MatchesTimestamp, the endTime is inclusive, because dates are
// discrete (though a half-open range would also work).
func MatchesDate(startDate, endDate, value string) bool {
	val, err := time.ParseInLocation(DateFormat, value, time.UTC)
	if err != nil {
		panic(err)
	}
	if startDate != `` {
		startDate, err := time.ParseInLocation(DateFormat, startDate, time.UTC)
		if err != nil {
			panic(err)
		}
		if val.Before(startDate) {
			return false
		}
	}
	if endDate != `` {
		endDate, err := time.ParseInLocation(DateFormat, endDate, time.UTC)
		if err != nil {
			panic(err)
		}
		if val.After(endDate) {
			return false
		}
	}
	return true
}

// WidenStartTime is a trivial implementation that truncates t by 24h.
// See also [WidenEndTime] and [WidenRange].
func WidenStartTime(t time.Time) time.Time {
	return t.Truncate(oneDay)
}

// WidenEndTime returns the next UTC midnight (start of day) after t, or t,
// if it is currently midnight, in the UTC time zone. In other words, it
// returns the timestamp by which the day is considered complete, i.e. no
// longer partial.
//
// See also [WidenStartTime] and [WidenRange].
func WidenEndTime(t time.Time) time.Time {
	// WARNING: Truncate operates on the 0 time (Go impl. detail).
	if truncated := t.Truncate(oneDay); !truncated.Equal(t) {
		return truncated.Add(oneDay) // next UTC midnight (start of day)
	}
	return t // UTC midnight (start of day)
}

// WidenRange is an alias for [WidenStartTime] and [WidenEndTime], is
// idempotent, and effectively moves the bounds of the range to include any
// overlapping days.
func WidenRange(start, end time.Time) (time.Time, time.Time) {
	return WidenStartTime(start), WidenEndTime(end)
}

// N.B. All the examples treat dates as normalised to 00:00:00 UTC.

func ExampleTimestampToDate(startTime, endTime time.Time) (startDate, endDate string) {
	if startTime != (time.Time{}) {
		// 1. Convert to UTC so values can be consistently converted in terms
		// of their actual meaning, without being affected by the zone/offset
		// of startTime
		startTime = startTime.UTC()

		// 2. Assume that dates are actually an approximate (rounded)
		// representation of time. In order to avoid matching partial days, and
		// therefore potentially matching the more than one period (for an
		// arbitrary interval), we need to round _up_ to the next day (to
		// narrow the range).
		if !startTime.Truncate(oneDay).Equal(startTime) {
			startTime = startTime.Add(oneDay)
		}

		// 3. The actual truncation is done here, in this implementation
		startDate = startTime.Format(DateFormat)
	}

	if endTime != (time.Time{}) {
		// 1. Convert to UTC, for the same reason as above
		endTime = endTime.UTC()

		// 2. Since we are converting from an open bound to a closed bound, we
		// need to adjust the end time to be exclusive (i.e. the previous day,
		// if the endTime was the start of the day, UTC)
		endTime = endTime.Add(-oneDay)

		// 3. The actual truncation is done here, in this implementation
		endDate = endTime.Format(DateFormat)
	}

	return
}

var _ TimestampToDate = ExampleTimestampToDate // compile-time type assertion (unnecessary)

func ExampleDateToTimestamp(startDate, endDate string) (startTime, endTime time.Time) {
	var err error

	if startDate != `` {
		// 1. Parse the date in UTC, because that is what they are documented
		// to represent (time will be start of day)
		startTime, err = time.ParseInLocation(DateFormat, startDate, time.UTC)
		if err != nil {
			panic(err)
		}
		// 2. Do nothing, because startTime will now correctly select from
		// startDate onwards (inclusive)
	}

	if endDate != `` {
		// 1. Parse in UTC, to get our initial timestamp
		endTime, err = time.ParseInLocation(DateFormat, endDate, time.UTC)
		if err != nil {
			panic(err)
		}
		// 2. Adjust, so that our endTime (exclusive) will correctly select
		// all instants within the original endDate (inclusive)
		endTime = endTime.Add(oneDay)
	}

	return
}

var _ DateToTimestamp = ExampleDateToTimestamp // compile-time type assertion (unnecessary)

// AssertDate ensures that s is a valid date.
func AssertDate(t *testing.T, s string) {
	t.Helper()
	d, err := time.ParseInLocation(DateFormat, s, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if d.Format(DateFormat) != s {
		t.Fatal(`date format mismatch`)
	}
}

// RangeTestCases is a utility for iterating on all combinations of ranges and
// values, including ranges where one side is set to the zero value.
func RangeTestCases(ranges [][2]string, values []string, f func(r [2]string, v string) bool) {
	seen := make(map[[3]string]struct{})
	for _, r := range ranges {
		for i := range 3 {
			r := r // N.B. seems not copied by default, even after recent loop var changes
			if i != 0 {
				r[i-1] = ``
			}
			for _, v := range values {
				{
					k := [3]string{r[0], r[1], v}
					if _, ok := seen[k]; ok {
						continue
					}
					seen[k] = struct{}{}
				}
				if !f(r, v) {
					return
				}
			}
		}
	}
}

// TestTimestampToDate may be used to test a [TimestampToDate] implementation.
// The ranges are timestamps, and the values are dates.
func TestTimestampToDate(t *testing.T, ranges [][2]string, values []string, matches map[[3]string]struct{}, convert TimestampToDate) {
	result := make(map[[3]string]struct{})
	setMatches := func(r [2]string, v string, matches bool) {
		k := [3]string{r[0], r[1], v}
		if matches {
			result[k] = struct{}{}
		} else {
			delete(result, k)
		}
	}

	t.Cleanup(func() {
		t.Logf(`actual matches: %s`,
			strings.NewReplacer(
				"[3]string{", "{",
				`struct {}{}`, `{}`,
				`struct{}{}`, `{}`,
			).Replace(fmt.Sprintf("%#v", result)))
	})

	RangeTestCases(ranges, values, func(r [2]string, value string) bool {
		t.Run(r[0]+`-`+r[1]+`-`+value, func(t *testing.T) {
			var startTime, endTime time.Time
			if r[0] != `` {
				var err error
				startTime, err = time.ParseInLocation(TimestampFormat, r[0], time.UTC)
				if err != nil {
					t.Fatal(err)
				}
			}
			if r[1] != `` {
				var err error
				endTime, err = time.ParseInLocation(TimestampFormat, r[1], time.UTC)
				if err != nil {
					t.Fatal(err)
				}
			}

			AssertDate(t, value)

			startDate, endDate := convert(startTime, endTime)
			if r[0] != `` {
				AssertDate(t, startDate)
			}
			if r[1] != `` {
				AssertDate(t, endDate)
			}

			actual := MatchesDate(startDate, endDate, value)

			setMatches(r, value, actual)

			if _, expected := matches[[3]string{r[0], r[1], value}]; actual != expected {
				t.Fatalf(`expected %t, got %t: [%s, %s] matching %s`, expected, actual, startDate, endDate, value)
			}
		})
		return true
	})
}

// TestDateToTimestamp may be used to test a [DateToTimestamp] implementation.
// The ranges are dates, and the values are timestamps.
func TestDateToTimestamp(t *testing.T, ranges [][2]string, values []string, matches map[[3]string]struct{}, convert DateToTimestamp) {
	result := make(map[[3]string]struct{})
	setMatches := func(r [2]string, v string, matches bool) {
		k := [3]string{r[0], r[1], v}
		if matches {
			result[k] = struct{}{}
		} else {
			delete(result, k)
		}
	}

	t.Cleanup(func() {
		t.Logf(`actual matches: %s`,
			strings.NewReplacer(
				"[3]string{", "{",
				`struct {}{}`, `{}`,
				`struct{}{}`, `{}`,
			).Replace(fmt.Sprintf("%#v", result)))
	})

	RangeTestCases(ranges, values, func(r [2]string, valStr string) bool {
		t.Run(r[0]+`-`+r[1]+`-`+valStr, func(t *testing.T) {
			value, err := time.ParseInLocation(TimestampFormat, valStr, time.UTC)
			if err != nil {
				t.Fatal(err)
			}

			if r[0] != `` {
				AssertDate(t, r[0])
			}
			if r[1] != `` {
				AssertDate(t, r[1])
			}

			startTime, endTime := convert(r[0], r[1])
			if (r[0] == ``) != (startTime == (time.Time{})) {
				t.Fatal(`start time zero value mismatch for input:`, r[0])
			}
			if (r[1] == ``) != (endTime == (time.Time{})) {
				t.Fatal(`end time time zero value mismatch for input:`, r[1])
			}

			actual := MatchesTimestamp(startTime, endTime, value)

			setMatches(r, valStr, actual)

			if _, expected := matches[[3]string{r[0], r[1], valStr}]; actual != expected {
				t.Fatalf(
					`expected %t, got %t: [%s, %s] matching %s`,
					expected,
					actual,
					startTime.Format(TimestampFormat),
					endTime.Format(TimestampFormat),
					value.Format(TimestampFormat),
				)
			}
		})
		return true
	})
}

func FuzzTimestampToDate(f *testing.F, ranges [][2]string, values []string, convert TimestampToDate) {
	offsetSecondsEastOfUTCValues := [...]int{math.MaxInt, -43200, -36000, -32400, -25200, -18000, -14400, -7200, 0, 3600, 7200, 14400, 18000, 25200, 32400, 43200}
	RangeTestCases(ranges, values, func(r [2]string, v string) bool {
		var startTime, endTime time.Time
		var err error
		if r[0] != `` {
			startTime, err = time.ParseInLocation(TimestampFormat, r[0], time.UTC)
			if err != nil {
				f.Fatal(err)
			}
		}
		if r[1] != `` {
			endTime, err = time.ParseInLocation(TimestampFormat, r[1], time.UTC)
			if err != nil {
				f.Fatal(err)
			}
		}
		value, err := time.ParseInLocation(DateFormat, v, time.UTC)
		if err != nil {
			f.Fatal(err)
		}
		for i, startOffset := range offsetSecondsEastOfUTCValues {
			if i == 0 {
				_, startOffset = startTime.Zone()
			}
			for j, endOffset := range offsetSecondsEastOfUTCValues {
				if j == 0 {
					_, endOffset = endTime.Zone()
				}
				f.Add(
					startTime.UnixNano(),
					startOffset,
					endTime.UnixNano(),
					endOffset,
					value.UnixNano(),
					startTime == (time.Time{}),
					endTime == (time.Time{}),
				)
			}
		}
		return true
	})
	f.Fuzz(func(t *testing.T, startTimeEpoch int64, startTimeOffset int, endTimeEpoch int64, endTimeOffset int, valueEpoch int64, ignoreStart, ignoreEnd bool) {
		if ignoreStart && ignoreEnd {
			t.Skip("skipping invalid range where both start and end are ignored")
		} else if !ignoreStart && !ignoreEnd && (startTimeEpoch >= endTimeEpoch || time.Duration(endTimeEpoch-startTimeEpoch) < 24*time.Hour) {
			t.Skipf("skipping invalid range where endTime (%s) is not at least 1 full day after startTime (%s)",
				time.Unix(0, startTimeEpoch).UTC().Format(TimestampFormat),
				time.Unix(0, endTimeEpoch).UTC().Format(TimestampFormat))
		}

		var startTime, endTime time.Time
		if !ignoreStart {
			startTime = time.Unix(0, startTimeEpoch).In(time.FixedZone("", startTimeOffset))
		}
		if !ignoreEnd {
			endTime = time.Unix(0, endTimeEpoch).In(time.FixedZone("", endTimeOffset))
		}

		value := time.Unix(0, valueEpoch).In(time.UTC).Format(DateFormat)

		startDate, endDate := convert(startTime, endTime)

		if ignoreStart != (startDate == ``) {
			t.Fatalf("ignoreStart=%t, startDate=%s", ignoreStart, startDate)
		}
		if ignoreEnd != (endDate == ``) {
			t.Fatalf("ignoreEnd=%t, endDate=%s", ignoreEnd, endDate)
		}

		matches := MatchesDate(startDate, endDate, value)

		var startDateParsed, endDateParsed time.Time
		var err error
		if !ignoreStart {
			startDateParsed, err = time.ParseInLocation(DateFormat, startDate, time.UTC)
			if err != nil || startDateParsed.Format(DateFormat) != startDate {
				t.Fatal(startDateParsed, err)
			}
		}
		if !ignoreEnd {
			endDateParsed, err = time.ParseInLocation(DateFormat, endDate, time.UTC)
			if err != nil || endDateParsed.Format(DateFormat) != endDate {
				t.Fatal(endDateParsed, err)
			}
		}
		if !ignoreStart && !ignoreEnd && startDateParsed.After(endDateParsed) {
			t.Fatalf("startDate is after endDate: startDate=%s (%s), endDate=%s (%s)",
				startDate, startTime.Format(TimestampFormat),
				endDate, endTime.Format(TimestampFormat))
		}

		// determine lower, and approximate inclusive upper bound for what would normalise to value
		valueLower, err := time.ParseInLocation(DateFormat, value, time.UTC)
		if err != nil {
			t.Fatal(err)
		}
		valueUpper := valueLower.Add(24*time.Hour - time.Nanosecond) // not actual upper, but upper representable here

		// the trivial cases for matching the original range
		valueLowerMatches := (ignoreStart || !startTime.After(valueLower)) &&
			(ignoreEnd || endTime.After(valueLower))
		valueUpperMatches := (ignoreStart || !startTime.After(valueUpper)) &&
			(ignoreEnd || endTime.After(valueUpper))

		// Both the upper and lower bound must match to be considered a match, otherwise the date isn't wholly
		// contained in the range. If we didn't handle matches this way, it may break "contiguous ranges".
		if matches != (valueUpperMatches && valueLowerMatches) {
			t.Fatalf(
				"expected %t, got (%t && %t):\ntimestamp range [%s, %s) -> date range [%s, %s]\n\tmatching\ndate value %s -> approx. timestamp value(s) between %s and %s (inclusive)",
				matches,
				valueLowerMatches,
				valueUpperMatches,
				startTime.Format(TimestampFormat),
				endTime.Format(TimestampFormat),
				startDate,
				endDate,
				value,
				valueLower.Format(TimestampFormat),
				valueUpper.Format(TimestampFormat),
			)
		}
	})
}

// DateValues are example date values, for testing purposes.
var DateValues = []string{
	"2024-01-01", // New Year's Day
	"2024-12-25", // Christmas
	"2024-02-29", // Leap year day
	"2024-07-04", // Independence Day
	"2024-11-05", // Random date
	"2024-06-15", // Another random date
	"2024-08-30", // Yet another random date
	"2024-10-31", // Halloween
	"2024-09-21", // Equinox
	"2024-03-10", // Daylight Saving Time starts (US)
	"2022-01-01",
	"2023-02-28",
	"2024-02-29", // leap year
	"2023-12-31",
	"2023-07-19",
	"2016-12-31", // leap second day
	"2023-03-12", // DST start in US
	"2023-11-05", // DST end in US
}

// DateRangeValues are example date range values, for testing purposes.
var DateRangeValues = [][2]string{
	{"2024-01-01", "2024-01-31"}, // Entire month of January
	{"2024-02-01", "2024-02-29"}, // Entire month of February (leap year)
	{"2024-07-01", "2024-07-04"}, // Independence week
	{"2024-12-24", "2024-12-26"}, // Christmas period
	{"2024-06-01", "2024-06-15"}, // First half of June
	{"2024-08-15", "2024-08-30"}, // Second half of August
	{"2024-10-01", "2024-10-31"}, // Entire month of October
	{"2024-09-20", "2024-09-22"}, // Around the Equinox
	{"2024-03-09", "2024-03-11"}, // Around Daylight Saving Time starts
	{"2024-11-01", "2024-11-05"}, // First days of November
	{"2022-01-01", "2022-01-31"},
	{"2023-02-01", "2023-02-28"},
	{"2024-02-01", "2024-02-29"}, // leap year
	{"2023-12-01", "2023-12-31"},
	{"2023-07-01", "2023-07-31"},
	{"2016-12-31", "2017-01-01"}, // leap second day range
	{"2023-03-10", "2023-03-12"}, // around DST start
	{"2023-11-04", "2023-11-06"}, // around DST end
}

// TimestampValues are example timestamp values, for testing purposes.
var TimestampValues = []string{
	"2024-01-01T00:00:00Z",      // New Year's Day UTC
	"2024-12-25T00:00:00-05:00", // Christmas EST
	"2024-02-29T12:00:00+05:30", // Leap year day IST
	"2024-07-04T23:59:59-07:00", // Independence Day PDT
	"2024-11-05T08:00:00+01:00", // Random date CET
	"2024-06-15T13:45:30+09:00", // Another random date JST
	"2024-08-30T18:30:00-04:00", // Yet another random date EDT
	"2024-10-31T17:00:00+00:00", // Halloween UTC
	"2024-09-21T00:00:00-03:00", // Equinox BRT
	"2024-03-10T02:00:00-08:00", // Daylight Saving Time starts PST
	"2022-01-01T00:00:00Z",
	"2023-02-28T23:59:59Z",
	"2024-02-29T12:00:00Z", // leap year
	"2023-12-31T23:59:59Z",
	"2023-07-19T14:30:00Z",
	"2023-07-19T14:30:00-07:00", // with offset
	"2023-07-19T14:30:00+09:00", // with offset
	"2023-03-12T02:00:00-07:00", // DST start in US
	"2023-11-05T01:00:00-08:00", // DST end in US
}

// TimestampRangeValues are example timestamp range values, for testing purposes.
var TimestampRangeValues = [][2]string{
	{"2024-01-01T00:00:00Z", "2024-01-31T23:59:59Z"},           // Entire month of January UTC
	{"2024-02-01T00:00:00-08:00", "2024-02-29T23:59:59-08:00"}, // Entire month of February PST (leap year)
	{"2024-07-01T00:00:00-07:00", "2024-07-04T23:59:59-07:00"}, // Independence week PDT
	{"2024-12-24T00:00:00+01:00", "2024-12-26T23:59:59+01:00"}, // Christmas period CET
	{"2024-06-01T00:00:00+09:00", "2024-06-15T23:59:59+09:00"}, // First half of June JST
	{"2024-08-15T00:00:00-04:00", "2024-08-30T23:59:59-04:00"}, // Second half of August EDT
	{"2024-10-01T00:00:00+00:00", "2024-10-31T23:59:59+00:00"}, // Entire month of October UTC
	{"2024-09-20T00:00:00-03:00", "2024-09-22T23:59:59-03:00"}, // Around the Equinox BRT
	{"2024-03-09T00:00:00-08:00", "2024-03-11T23:59:59-07:00"}, // Around Daylight Saving Time starts (PST to PDT)
	{"2024-11-01T00:00:00+01:00", "2024-11-05T23:59:59+01:00"}, // First days of November CET
	{"2022-01-01T00:00:00Z", "2022-01-31T23:59:59Z"},
	{"2023-02-01T00:00:00Z", "2023-02-28T23:59:59Z"},
	{"2024-02-01T00:00:00Z", "2024-02-29T23:59:59Z"}, // leap year
	{"2023-12-01T00:00:00Z", "2023-12-31T23:59:59Z"},
	{"2023-07-01T00:00:00Z", "2023-07-31T23:59:59Z"},
	{"2023-07-01T00:00:00-07:00", "2023-07-31T23:59:59-07:00"}, // with offset
	{"2023-07-01T00:00:00+09:00", "2023-07-31T23:59:59+09:00"}, // with offset
	{"2016-12-31T23:59:59Z", "2017-01-01T00:00:00Z"},           // around leap second
	{"2023-03-12T01:59:59-07:00", "2023-03-12T03:00:00-07:00"}, // around DST start
	{"2023-11-05T00:59:59-07:00", "2023-11-05T02:00:00-08:00"}, // around DST end
}

// ExampleMatches are all tuples like (range_i, range_j, value_k), which return
// true, from their corresponding/appropriate comparison function, where
// value_k differs in format (same-format is uninteresting).
var ExampleMatches = map[[3]string]struct{}{
	// for TimestampToDate

	{"", "2017-01-01T00:00:00Z", "2016-12-31"}:                               {},
	{"", "2022-01-31T23:59:59Z", "2016-12-31"}:                               {},
	{"", "2022-01-31T23:59:59Z", "2022-01-01"}:                               {},
	{"", "2023-02-28T23:59:59Z", "2016-12-31"}:                               {},
	{"", "2023-02-28T23:59:59Z", "2022-01-01"}:                               {},
	{"", "2023-03-12T03:00:00-07:00", "2016-12-31"}:                          {},
	{"", "2023-03-12T03:00:00-07:00", "2022-01-01"}:                          {},
	{"", "2023-03-12T03:00:00-07:00", "2023-02-28"}:                          {},
	{"", "2023-07-31T23:59:59+09:00", "2016-12-31"}:                          {},
	{"", "2023-07-31T23:59:59+09:00", "2022-01-01"}:                          {},
	{"", "2023-07-31T23:59:59+09:00", "2023-02-28"}:                          {},
	{"", "2023-07-31T23:59:59+09:00", "2023-03-12"}:                          {},
	{"", "2023-07-31T23:59:59+09:00", "2023-07-19"}:                          {},
	{"", "2023-07-31T23:59:59-07:00", "2016-12-31"}:                          {},
	{"", "2023-07-31T23:59:59-07:00", "2022-01-01"}:                          {},
	{"", "2023-07-31T23:59:59-07:00", "2023-02-28"}:                          {},
	{"", "2023-07-31T23:59:59-07:00", "2023-03-12"}:                          {},
	{"", "2023-07-31T23:59:59-07:00", "2023-07-19"}:                          {},
	{"", "2023-07-31T23:59:59Z", "2016-12-31"}:                               {},
	{"", "2023-07-31T23:59:59Z", "2022-01-01"}:                               {},
	{"", "2023-07-31T23:59:59Z", "2023-02-28"}:                               {},
	{"", "2023-07-31T23:59:59Z", "2023-03-12"}:                               {},
	{"", "2023-07-31T23:59:59Z", "2023-07-19"}:                               {},
	{"", "2023-11-05T02:00:00-08:00", "2016-12-31"}:                          {},
	{"", "2023-11-05T02:00:00-08:00", "2022-01-01"}:                          {},
	{"", "2023-11-05T02:00:00-08:00", "2023-02-28"}:                          {},
	{"", "2023-11-05T02:00:00-08:00", "2023-03-12"}:                          {},
	{"", "2023-11-05T02:00:00-08:00", "2023-07-19"}:                          {},
	{"", "2023-12-31T23:59:59Z", "2016-12-31"}:                               {},
	{"", "2023-12-31T23:59:59Z", "2022-01-01"}:                               {},
	{"", "2023-12-31T23:59:59Z", "2023-02-28"}:                               {},
	{"", "2023-12-31T23:59:59Z", "2023-03-12"}:                               {},
	{"", "2023-12-31T23:59:59Z", "2023-07-19"}:                               {},
	{"", "2023-12-31T23:59:59Z", "2023-11-05"}:                               {},
	{"", "2024-01-31T23:59:59Z", "2016-12-31"}:                               {},
	{"", "2024-01-31T23:59:59Z", "2022-01-01"}:                               {},
	{"", "2024-01-31T23:59:59Z", "2023-02-28"}:                               {},
	{"", "2024-01-31T23:59:59Z", "2023-03-12"}:                               {},
	{"", "2024-01-31T23:59:59Z", "2023-07-19"}:                               {},
	{"", "2024-01-31T23:59:59Z", "2023-11-05"}:                               {},
	{"", "2024-01-31T23:59:59Z", "2023-12-31"}:                               {},
	{"", "2024-01-31T23:59:59Z", "2024-01-01"}:                               {},
	{"", "2024-02-29T23:59:59-08:00", "2016-12-31"}:                          {},
	{"", "2024-02-29T23:59:59-08:00", "2022-01-01"}:                          {},
	{"", "2024-02-29T23:59:59-08:00", "2023-02-28"}:                          {},
	{"", "2024-02-29T23:59:59-08:00", "2023-03-12"}:                          {},
	{"", "2024-02-29T23:59:59-08:00", "2023-07-19"}:                          {},
	{"", "2024-02-29T23:59:59-08:00", "2023-11-05"}:                          {},
	{"", "2024-02-29T23:59:59-08:00", "2023-12-31"}:                          {},
	{"", "2024-02-29T23:59:59-08:00", "2024-01-01"}:                          {},
	{"", "2024-02-29T23:59:59-08:00", "2024-02-29"}:                          {},
	{"", "2024-02-29T23:59:59Z", "2016-12-31"}:                               {},
	{"", "2024-02-29T23:59:59Z", "2022-01-01"}:                               {},
	{"", "2024-02-29T23:59:59Z", "2023-02-28"}:                               {},
	{"", "2024-02-29T23:59:59Z", "2023-03-12"}:                               {},
	{"", "2024-02-29T23:59:59Z", "2023-07-19"}:                               {},
	{"", "2024-02-29T23:59:59Z", "2023-11-05"}:                               {},
	{"", "2024-02-29T23:59:59Z", "2023-12-31"}:                               {},
	{"", "2024-02-29T23:59:59Z", "2024-01-01"}:                               {},
	{"", "2024-03-11T23:59:59-07:00", "2016-12-31"}:                          {},
	{"", "2024-03-11T23:59:59-07:00", "2022-01-01"}:                          {},
	{"", "2024-03-11T23:59:59-07:00", "2023-02-28"}:                          {},
	{"", "2024-03-11T23:59:59-07:00", "2023-03-12"}:                          {},
	{"", "2024-03-11T23:59:59-07:00", "2023-07-19"}:                          {},
	{"", "2024-03-11T23:59:59-07:00", "2023-11-05"}:                          {},
	{"", "2024-03-11T23:59:59-07:00", "2023-12-31"}:                          {},
	{"", "2024-03-11T23:59:59-07:00", "2024-01-01"}:                          {},
	{"", "2024-03-11T23:59:59-07:00", "2024-02-29"}:                          {},
	{"", "2024-03-11T23:59:59-07:00", "2024-03-10"}:                          {},
	{"", "2024-06-15T23:59:59+09:00", "2016-12-31"}:                          {},
	{"", "2024-06-15T23:59:59+09:00", "2022-01-01"}:                          {},
	{"", "2024-06-15T23:59:59+09:00", "2023-02-28"}:                          {},
	{"", "2024-06-15T23:59:59+09:00", "2023-03-12"}:                          {},
	{"", "2024-06-15T23:59:59+09:00", "2023-07-19"}:                          {},
	{"", "2024-06-15T23:59:59+09:00", "2023-11-05"}:                          {},
	{"", "2024-06-15T23:59:59+09:00", "2023-12-31"}:                          {},
	{"", "2024-06-15T23:59:59+09:00", "2024-01-01"}:                          {},
	{"", "2024-06-15T23:59:59+09:00", "2024-02-29"}:                          {},
	{"", "2024-06-15T23:59:59+09:00", "2024-03-10"}:                          {},
	{"", "2024-07-04T23:59:59-07:00", "2016-12-31"}:                          {},
	{"", "2024-07-04T23:59:59-07:00", "2022-01-01"}:                          {},
	{"", "2024-07-04T23:59:59-07:00", "2023-02-28"}:                          {},
	{"", "2024-07-04T23:59:59-07:00", "2023-03-12"}:                          {},
	{"", "2024-07-04T23:59:59-07:00", "2023-07-19"}:                          {},
	{"", "2024-07-04T23:59:59-07:00", "2023-11-05"}:                          {},
	{"", "2024-07-04T23:59:59-07:00", "2023-12-31"}:                          {},
	{"", "2024-07-04T23:59:59-07:00", "2024-01-01"}:                          {},
	{"", "2024-07-04T23:59:59-07:00", "2024-02-29"}:                          {},
	{"", "2024-07-04T23:59:59-07:00", "2024-03-10"}:                          {},
	{"", "2024-07-04T23:59:59-07:00", "2024-06-15"}:                          {},
	{"", "2024-07-04T23:59:59-07:00", "2024-07-04"}:                          {},
	{"", "2024-08-30T23:59:59-04:00", "2016-12-31"}:                          {},
	{"", "2024-08-30T23:59:59-04:00", "2022-01-01"}:                          {},
	{"", "2024-08-30T23:59:59-04:00", "2023-02-28"}:                          {},
	{"", "2024-08-30T23:59:59-04:00", "2023-03-12"}:                          {},
	{"", "2024-08-30T23:59:59-04:00", "2023-07-19"}:                          {},
	{"", "2024-08-30T23:59:59-04:00", "2023-11-05"}:                          {},
	{"", "2024-08-30T23:59:59-04:00", "2023-12-31"}:                          {},
	{"", "2024-08-30T23:59:59-04:00", "2024-01-01"}:                          {},
	{"", "2024-08-30T23:59:59-04:00", "2024-02-29"}:                          {},
	{"", "2024-08-30T23:59:59-04:00", "2024-03-10"}:                          {},
	{"", "2024-08-30T23:59:59-04:00", "2024-06-15"}:                          {},
	{"", "2024-08-30T23:59:59-04:00", "2024-07-04"}:                          {},
	{"", "2024-08-30T23:59:59-04:00", "2024-08-30"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2016-12-31"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2022-01-01"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2023-02-28"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2023-03-12"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2023-07-19"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2023-11-05"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2023-12-31"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2024-01-01"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2024-02-29"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2024-03-10"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2024-06-15"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2024-07-04"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2024-08-30"}:                          {},
	{"", "2024-09-22T23:59:59-03:00", "2024-09-21"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2016-12-31"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2022-01-01"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2023-02-28"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2023-03-12"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2023-07-19"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2023-11-05"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2023-12-31"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2024-01-01"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2024-02-29"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2024-03-10"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2024-06-15"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2024-07-04"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2024-08-30"}:                          {},
	{"", "2024-10-31T23:59:59+00:00", "2024-09-21"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2016-12-31"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2022-01-01"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2023-02-28"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2023-03-12"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2023-07-19"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2023-11-05"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2023-12-31"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2024-01-01"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2024-02-29"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2024-03-10"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2024-06-15"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2024-07-04"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2024-08-30"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2024-09-21"}:                          {},
	{"", "2024-11-05T23:59:59+01:00", "2024-10-31"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2016-12-31"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2022-01-01"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2023-02-28"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2023-03-12"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2023-07-19"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2023-11-05"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2023-12-31"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2024-01-01"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2024-02-29"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2024-03-10"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2024-06-15"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2024-07-04"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2024-08-30"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2024-09-21"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2024-10-31"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2024-11-05"}:                          {},
	{"", "2024-12-26T23:59:59+01:00", "2024-12-25"}:                          {},
	{"2016-12-31T23:59:59Z", "", "2022-01-01"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2023-02-28"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2023-03-12"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2023-07-19"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2023-11-05"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2023-12-31"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2024-01-01"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2024-02-29"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2024-03-10"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2024-06-15"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2024-07-04"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2024-08-30"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2024-09-21"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2024-10-31"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2024-11-05"}:                               {},
	{"2016-12-31T23:59:59Z", "", "2024-12-25"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2022-01-01"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2023-02-28"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2023-03-12"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2023-07-19"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2023-11-05"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2023-12-31"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2024-01-01"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2024-02-29"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2024-03-10"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2024-06-15"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2024-07-04"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2024-08-30"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2024-09-21"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2024-10-31"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2024-11-05"}:                               {},
	{"2022-01-01T00:00:00Z", "", "2024-12-25"}:                               {},
	{"2022-01-01T00:00:00Z", "2022-01-31T23:59:59Z", "2022-01-01"}:           {},
	{"2023-02-01T00:00:00Z", "", "2023-02-28"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2023-03-12"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2023-07-19"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2023-11-05"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2023-12-31"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2024-01-01"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2024-02-29"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2024-03-10"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2024-06-15"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2024-07-04"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2024-08-30"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2024-09-21"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2024-10-31"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2024-11-05"}:                               {},
	{"2023-02-01T00:00:00Z", "", "2024-12-25"}:                               {},
	{"2023-03-12T01:59:59-07:00", "", "2023-07-19"}:                          {},
	{"2023-03-12T01:59:59-07:00", "", "2023-11-05"}:                          {},
	{"2023-03-12T01:59:59-07:00", "", "2023-12-31"}:                          {},
	{"2023-03-12T01:59:59-07:00", "", "2024-01-01"}:                          {},
	{"2023-03-12T01:59:59-07:00", "", "2024-02-29"}:                          {},
	{"2023-03-12T01:59:59-07:00", "", "2024-03-10"}:                          {},
	{"2023-03-12T01:59:59-07:00", "", "2024-06-15"}:                          {},
	{"2023-03-12T01:59:59-07:00", "", "2024-07-04"}:                          {},
	{"2023-03-12T01:59:59-07:00", "", "2024-08-30"}:                          {},
	{"2023-03-12T01:59:59-07:00", "", "2024-09-21"}:                          {},
	{"2023-03-12T01:59:59-07:00", "", "2024-10-31"}:                          {},
	{"2023-03-12T01:59:59-07:00", "", "2024-11-05"}:                          {},
	{"2023-03-12T01:59:59-07:00", "", "2024-12-25"}:                          {},
	{"2023-07-01T00:00:00+09:00", "", "2023-07-19"}:                          {},
	{"2023-07-01T00:00:00+09:00", "", "2023-11-05"}:                          {},
	{"2023-07-01T00:00:00+09:00", "", "2023-12-31"}:                          {},
	{"2023-07-01T00:00:00+09:00", "", "2024-01-01"}:                          {},
	{"2023-07-01T00:00:00+09:00", "", "2024-02-29"}:                          {},
	{"2023-07-01T00:00:00+09:00", "", "2024-03-10"}:                          {},
	{"2023-07-01T00:00:00+09:00", "", "2024-06-15"}:                          {},
	{"2023-07-01T00:00:00+09:00", "", "2024-07-04"}:                          {},
	{"2023-07-01T00:00:00+09:00", "", "2024-08-30"}:                          {},
	{"2023-07-01T00:00:00+09:00", "", "2024-09-21"}:                          {},
	{"2023-07-01T00:00:00+09:00", "", "2024-10-31"}:                          {},
	{"2023-07-01T00:00:00+09:00", "", "2024-11-05"}:                          {},
	{"2023-07-01T00:00:00+09:00", "", "2024-12-25"}:                          {},
	{"2023-07-01T00:00:00+09:00", "2023-07-31T23:59:59+09:00", "2023-07-19"}: {},
	{"2023-07-01T00:00:00-07:00", "", "2023-07-19"}:                          {},
	{"2023-07-01T00:00:00-07:00", "", "2023-11-05"}:                          {},
	{"2023-07-01T00:00:00-07:00", "", "2023-12-31"}:                          {},
	{"2023-07-01T00:00:00-07:00", "", "2024-01-01"}:                          {},
	{"2023-07-01T00:00:00-07:00", "", "2024-02-29"}:                          {},
	{"2023-07-01T00:00:00-07:00", "", "2024-03-10"}:                          {},
	{"2023-07-01T00:00:00-07:00", "", "2024-06-15"}:                          {},
	{"2023-07-01T00:00:00-07:00", "", "2024-07-04"}:                          {},
	{"2023-07-01T00:00:00-07:00", "", "2024-08-30"}:                          {},
	{"2023-07-01T00:00:00-07:00", "", "2024-09-21"}:                          {},
	{"2023-07-01T00:00:00-07:00", "", "2024-10-31"}:                          {},
	{"2023-07-01T00:00:00-07:00", "", "2024-11-05"}:                          {},
	{"2023-07-01T00:00:00-07:00", "", "2024-12-25"}:                          {},
	{"2023-07-01T00:00:00-07:00", "2023-07-31T23:59:59-07:00", "2023-07-19"}: {},
	{"2023-07-01T00:00:00Z", "", "2023-07-19"}:                               {},
	{"2023-07-01T00:00:00Z", "", "2023-11-05"}:                               {},
	{"2023-07-01T00:00:00Z", "", "2023-12-31"}:                               {},
	{"2023-07-01T00:00:00Z", "", "2024-01-01"}:                               {},
	{"2023-07-01T00:00:00Z", "", "2024-02-29"}:                               {},
	{"2023-07-01T00:00:00Z", "", "2024-03-10"}:                               {},
	{"2023-07-01T00:00:00Z", "", "2024-06-15"}:                               {},
	{"2023-07-01T00:00:00Z", "", "2024-07-04"}:                               {},
	{"2023-07-01T00:00:00Z", "", "2024-08-30"}:                               {},
	{"2023-07-01T00:00:00Z", "", "2024-09-21"}:                               {},
	{"2023-07-01T00:00:00Z", "", "2024-10-31"}:                               {},
	{"2023-07-01T00:00:00Z", "", "2024-11-05"}:                               {},
	{"2023-07-01T00:00:00Z", "", "2024-12-25"}:                               {},
	{"2023-07-01T00:00:00Z", "2023-07-31T23:59:59Z", "2023-07-19"}:           {},
	{"2023-11-05T00:59:59-07:00", "", "2023-12-31"}:                          {},
	{"2023-11-05T00:59:59-07:00", "", "2024-01-01"}:                          {},
	{"2023-11-05T00:59:59-07:00", "", "2024-02-29"}:                          {},
	{"2023-11-05T00:59:59-07:00", "", "2024-03-10"}:                          {},
	{"2023-11-05T00:59:59-07:00", "", "2024-06-15"}:                          {},
	{"2023-11-05T00:59:59-07:00", "", "2024-07-04"}:                          {},
	{"2023-11-05T00:59:59-07:00", "", "2024-08-30"}:                          {},
	{"2023-11-05T00:59:59-07:00", "", "2024-09-21"}:                          {},
	{"2023-11-05T00:59:59-07:00", "", "2024-10-31"}:                          {},
	{"2023-11-05T00:59:59-07:00", "", "2024-11-05"}:                          {},
	{"2023-11-05T00:59:59-07:00", "", "2024-12-25"}:                          {},
	{"2023-12-01T00:00:00Z", "", "2023-12-31"}:                               {},
	{"2023-12-01T00:00:00Z", "", "2024-01-01"}:                               {},
	{"2023-12-01T00:00:00Z", "", "2024-02-29"}:                               {},
	{"2023-12-01T00:00:00Z", "", "2024-03-10"}:                               {},
	{"2023-12-01T00:00:00Z", "", "2024-06-15"}:                               {},
	{"2023-12-01T00:00:00Z", "", "2024-07-04"}:                               {},
	{"2023-12-01T00:00:00Z", "", "2024-08-30"}:                               {},
	{"2023-12-01T00:00:00Z", "", "2024-09-21"}:                               {},
	{"2023-12-01T00:00:00Z", "", "2024-10-31"}:                               {},
	{"2023-12-01T00:00:00Z", "", "2024-11-05"}:                               {},
	{"2023-12-01T00:00:00Z", "", "2024-12-25"}:                               {},
	{"2024-01-01T00:00:00Z", "", "2024-01-01"}:                               {},
	{"2024-01-01T00:00:00Z", "", "2024-02-29"}:                               {},
	{"2024-01-01T00:00:00Z", "", "2024-03-10"}:                               {},
	{"2024-01-01T00:00:00Z", "", "2024-06-15"}:                               {},
	{"2024-01-01T00:00:00Z", "", "2024-07-04"}:                               {},
	{"2024-01-01T00:00:00Z", "", "2024-08-30"}:                               {},
	{"2024-01-01T00:00:00Z", "", "2024-09-21"}:                               {},
	{"2024-01-01T00:00:00Z", "", "2024-10-31"}:                               {},
	{"2024-01-01T00:00:00Z", "", "2024-11-05"}:                               {},
	{"2024-01-01T00:00:00Z", "", "2024-12-25"}:                               {},
	{"2024-01-01T00:00:00Z", "2024-01-31T23:59:59Z", "2024-01-01"}:           {},
	{"2024-02-01T00:00:00-08:00", "", "2024-02-29"}:                          {},
	{"2024-02-01T00:00:00-08:00", "", "2024-03-10"}:                          {},
	{"2024-02-01T00:00:00-08:00", "", "2024-06-15"}:                          {},
	{"2024-02-01T00:00:00-08:00", "", "2024-07-04"}:                          {},
	{"2024-02-01T00:00:00-08:00", "", "2024-08-30"}:                          {},
	{"2024-02-01T00:00:00-08:00", "", "2024-09-21"}:                          {},
	{"2024-02-01T00:00:00-08:00", "", "2024-10-31"}:                          {},
	{"2024-02-01T00:00:00-08:00", "", "2024-11-05"}:                          {},
	{"2024-02-01T00:00:00-08:00", "", "2024-12-25"}:                          {},
	{"2024-02-01T00:00:00-08:00", "2024-02-29T23:59:59-08:00", "2024-02-29"}: {},
	{"2024-02-01T00:00:00Z", "", "2024-02-29"}:                               {},
	{"2024-02-01T00:00:00Z", "", "2024-03-10"}:                               {},
	{"2024-02-01T00:00:00Z", "", "2024-06-15"}:                               {},
	{"2024-02-01T00:00:00Z", "", "2024-07-04"}:                               {},
	{"2024-02-01T00:00:00Z", "", "2024-08-30"}:                               {},
	{"2024-02-01T00:00:00Z", "", "2024-09-21"}:                               {},
	{"2024-02-01T00:00:00Z", "", "2024-10-31"}:                               {},
	{"2024-02-01T00:00:00Z", "", "2024-11-05"}:                               {},
	{"2024-02-01T00:00:00Z", "", "2024-12-25"}:                               {},
	{"2024-03-09T00:00:00-08:00", "", "2024-03-10"}:                          {},
	{"2024-03-09T00:00:00-08:00", "", "2024-06-15"}:                          {},
	{"2024-03-09T00:00:00-08:00", "", "2024-07-04"}:                          {},
	{"2024-03-09T00:00:00-08:00", "", "2024-08-30"}:                          {},
	{"2024-03-09T00:00:00-08:00", "", "2024-09-21"}:                          {},
	{"2024-03-09T00:00:00-08:00", "", "2024-10-31"}:                          {},
	{"2024-03-09T00:00:00-08:00", "", "2024-11-05"}:                          {},
	{"2024-03-09T00:00:00-08:00", "", "2024-12-25"}:                          {},
	{"2024-03-09T00:00:00-08:00", "2024-03-11T23:59:59-07:00", "2024-03-10"}: {},
	{"2024-06-01T00:00:00+09:00", "", "2024-06-15"}:                          {},
	{"2024-06-01T00:00:00+09:00", "", "2024-07-04"}:                          {},
	{"2024-06-01T00:00:00+09:00", "", "2024-08-30"}:                          {},
	{"2024-06-01T00:00:00+09:00", "", "2024-09-21"}:                          {},
	{"2024-06-01T00:00:00+09:00", "", "2024-10-31"}:                          {},
	{"2024-06-01T00:00:00+09:00", "", "2024-11-05"}:                          {},
	{"2024-06-01T00:00:00+09:00", "", "2024-12-25"}:                          {},
	{"2024-07-01T00:00:00-07:00", "", "2024-07-04"}:                          {},
	{"2024-07-01T00:00:00-07:00", "", "2024-08-30"}:                          {},
	{"2024-07-01T00:00:00-07:00", "", "2024-09-21"}:                          {},
	{"2024-07-01T00:00:00-07:00", "", "2024-10-31"}:                          {},
	{"2024-07-01T00:00:00-07:00", "", "2024-11-05"}:                          {},
	{"2024-07-01T00:00:00-07:00", "", "2024-12-25"}:                          {},
	{"2024-07-01T00:00:00-07:00", "2024-07-04T23:59:59-07:00", "2024-07-04"}: {},
	{"2024-08-15T00:00:00-04:00", "", "2024-08-30"}:                          {},
	{"2024-08-15T00:00:00-04:00", "", "2024-09-21"}:                          {},
	{"2024-08-15T00:00:00-04:00", "", "2024-10-31"}:                          {},
	{"2024-08-15T00:00:00-04:00", "", "2024-11-05"}:                          {},
	{"2024-08-15T00:00:00-04:00", "", "2024-12-25"}:                          {},
	{"2024-08-15T00:00:00-04:00", "2024-08-30T23:59:59-04:00", "2024-08-30"}: {},
	{"2024-09-20T00:00:00-03:00", "", "2024-09-21"}:                          {},
	{"2024-09-20T00:00:00-03:00", "", "2024-10-31"}:                          {},
	{"2024-09-20T00:00:00-03:00", "", "2024-11-05"}:                          {},
	{"2024-09-20T00:00:00-03:00", "", "2024-12-25"}:                          {},
	{"2024-09-20T00:00:00-03:00", "2024-09-22T23:59:59-03:00", "2024-09-21"}: {},
	{"2024-10-01T00:00:00+00:00", "", "2024-10-31"}:                          {},
	{"2024-10-01T00:00:00+00:00", "", "2024-11-05"}:                          {},
	{"2024-10-01T00:00:00+00:00", "", "2024-12-25"}:                          {},
	{"2024-11-01T00:00:00+01:00", "", "2024-11-05"}:                          {},
	{"2024-11-01T00:00:00+01:00", "", "2024-12-25"}:                          {},
	{"2024-12-24T00:00:00+01:00", "", "2024-12-25"}:                          {},
	{"2024-12-24T00:00:00+01:00", "2024-12-26T23:59:59+01:00", "2024-12-25"}: {},

	// for DateToTimestamp

	{"", "2022-01-31", "2022-01-01T00:00:00Z"}:                {},
	{"", "2023-02-28", "2022-01-01T00:00:00Z"}:                {},
	{"", "2023-02-28", "2023-02-28T23:59:59Z"}:                {},
	{"", "2023-03-12", "2022-01-01T00:00:00Z"}:                {},
	{"", "2023-03-12", "2023-02-28T23:59:59Z"}:                {},
	{"", "2023-03-12", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2023-07-31", "2022-01-01T00:00:00Z"}:                {},
	{"", "2023-07-31", "2023-02-28T23:59:59Z"}:                {},
	{"", "2023-07-31", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2023-07-31", "2023-07-19T14:30:00+09:00"}:           {},
	{"", "2023-07-31", "2023-07-19T14:30:00-07:00"}:           {},
	{"", "2023-07-31", "2023-07-19T14:30:00Z"}:                {},
	{"", "2023-11-06", "2022-01-01T00:00:00Z"}:                {},
	{"", "2023-11-06", "2023-02-28T23:59:59Z"}:                {},
	{"", "2023-11-06", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2023-11-06", "2023-07-19T14:30:00+09:00"}:           {},
	{"", "2023-11-06", "2023-07-19T14:30:00-07:00"}:           {},
	{"", "2023-11-06", "2023-07-19T14:30:00Z"}:                {},
	{"", "2023-11-06", "2023-11-05T01:00:00-08:00"}:           {},
	{"", "2023-12-31", "2022-01-01T00:00:00Z"}:                {},
	{"", "2023-12-31", "2023-02-28T23:59:59Z"}:                {},
	{"", "2023-12-31", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2023-12-31", "2023-07-19T14:30:00+09:00"}:           {},
	{"", "2023-12-31", "2023-07-19T14:30:00-07:00"}:           {},
	{"", "2023-12-31", "2023-07-19T14:30:00Z"}:                {},
	{"", "2023-12-31", "2023-11-05T01:00:00-08:00"}:           {},
	{"", "2023-12-31", "2023-12-31T23:59:59Z"}:                {},
	{"", "2024-01-31", "2022-01-01T00:00:00Z"}:                {},
	{"", "2024-01-31", "2023-02-28T23:59:59Z"}:                {},
	{"", "2024-01-31", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2024-01-31", "2023-07-19T14:30:00+09:00"}:           {},
	{"", "2024-01-31", "2023-07-19T14:30:00-07:00"}:           {},
	{"", "2024-01-31", "2023-07-19T14:30:00Z"}:                {},
	{"", "2024-01-31", "2023-11-05T01:00:00-08:00"}:           {},
	{"", "2024-01-31", "2023-12-31T23:59:59Z"}:                {},
	{"", "2024-01-31", "2024-01-01T00:00:00Z"}:                {},
	{"", "2024-02-29", "2022-01-01T00:00:00Z"}:                {},
	{"", "2024-02-29", "2023-02-28T23:59:59Z"}:                {},
	{"", "2024-02-29", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2024-02-29", "2023-07-19T14:30:00+09:00"}:           {},
	{"", "2024-02-29", "2023-07-19T14:30:00-07:00"}:           {},
	{"", "2024-02-29", "2023-07-19T14:30:00Z"}:                {},
	{"", "2024-02-29", "2023-11-05T01:00:00-08:00"}:           {},
	{"", "2024-02-29", "2023-12-31T23:59:59Z"}:                {},
	{"", "2024-02-29", "2024-01-01T00:00:00Z"}:                {},
	{"", "2024-02-29", "2024-02-29T12:00:00+05:30"}:           {},
	{"", "2024-02-29", "2024-02-29T12:00:00Z"}:                {},
	{"", "2024-03-11", "2022-01-01T00:00:00Z"}:                {},
	{"", "2024-03-11", "2023-02-28T23:59:59Z"}:                {},
	{"", "2024-03-11", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2024-03-11", "2023-07-19T14:30:00+09:00"}:           {},
	{"", "2024-03-11", "2023-07-19T14:30:00-07:00"}:           {},
	{"", "2024-03-11", "2023-07-19T14:30:00Z"}:                {},
	{"", "2024-03-11", "2023-11-05T01:00:00-08:00"}:           {},
	{"", "2024-03-11", "2023-12-31T23:59:59Z"}:                {},
	{"", "2024-03-11", "2024-01-01T00:00:00Z"}:                {},
	{"", "2024-03-11", "2024-02-29T12:00:00+05:30"}:           {},
	{"", "2024-03-11", "2024-02-29T12:00:00Z"}:                {},
	{"", "2024-03-11", "2024-03-10T02:00:00-08:00"}:           {},
	{"", "2024-06-15", "2022-01-01T00:00:00Z"}:                {},
	{"", "2024-06-15", "2023-02-28T23:59:59Z"}:                {},
	{"", "2024-06-15", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2024-06-15", "2023-07-19T14:30:00+09:00"}:           {},
	{"", "2024-06-15", "2023-07-19T14:30:00-07:00"}:           {},
	{"", "2024-06-15", "2023-07-19T14:30:00Z"}:                {},
	{"", "2024-06-15", "2023-11-05T01:00:00-08:00"}:           {},
	{"", "2024-06-15", "2023-12-31T23:59:59Z"}:                {},
	{"", "2024-06-15", "2024-01-01T00:00:00Z"}:                {},
	{"", "2024-06-15", "2024-02-29T12:00:00+05:30"}:           {},
	{"", "2024-06-15", "2024-02-29T12:00:00Z"}:                {},
	{"", "2024-06-15", "2024-03-10T02:00:00-08:00"}:           {},
	{"", "2024-06-15", "2024-06-15T13:45:30+09:00"}:           {},
	{"", "2024-07-04", "2022-01-01T00:00:00Z"}:                {},
	{"", "2024-07-04", "2023-02-28T23:59:59Z"}:                {},
	{"", "2024-07-04", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2024-07-04", "2023-07-19T14:30:00+09:00"}:           {},
	{"", "2024-07-04", "2023-07-19T14:30:00-07:00"}:           {},
	{"", "2024-07-04", "2023-07-19T14:30:00Z"}:                {},
	{"", "2024-07-04", "2023-11-05T01:00:00-08:00"}:           {},
	{"", "2024-07-04", "2023-12-31T23:59:59Z"}:                {},
	{"", "2024-07-04", "2024-01-01T00:00:00Z"}:                {},
	{"", "2024-07-04", "2024-02-29T12:00:00+05:30"}:           {},
	{"", "2024-07-04", "2024-02-29T12:00:00Z"}:                {},
	{"", "2024-07-04", "2024-03-10T02:00:00-08:00"}:           {},
	{"", "2024-07-04", "2024-06-15T13:45:30+09:00"}:           {},
	{"", "2024-08-30", "2022-01-01T00:00:00Z"}:                {},
	{"", "2024-08-30", "2023-02-28T23:59:59Z"}:                {},
	{"", "2024-08-30", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2024-08-30", "2023-07-19T14:30:00+09:00"}:           {},
	{"", "2024-08-30", "2023-07-19T14:30:00-07:00"}:           {},
	{"", "2024-08-30", "2023-07-19T14:30:00Z"}:                {},
	{"", "2024-08-30", "2023-11-05T01:00:00-08:00"}:           {},
	{"", "2024-08-30", "2023-12-31T23:59:59Z"}:                {},
	{"", "2024-08-30", "2024-01-01T00:00:00Z"}:                {},
	{"", "2024-08-30", "2024-02-29T12:00:00+05:30"}:           {},
	{"", "2024-08-30", "2024-02-29T12:00:00Z"}:                {},
	{"", "2024-08-30", "2024-03-10T02:00:00-08:00"}:           {},
	{"", "2024-08-30", "2024-06-15T13:45:30+09:00"}:           {},
	{"", "2024-08-30", "2024-07-04T23:59:59-07:00"}:           {},
	{"", "2024-08-30", "2024-08-30T18:30:00-04:00"}:           {},
	{"", "2024-09-22", "2022-01-01T00:00:00Z"}:                {},
	{"", "2024-09-22", "2023-02-28T23:59:59Z"}:                {},
	{"", "2024-09-22", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2024-09-22", "2023-07-19T14:30:00+09:00"}:           {},
	{"", "2024-09-22", "2023-07-19T14:30:00-07:00"}:           {},
	{"", "2024-09-22", "2023-07-19T14:30:00Z"}:                {},
	{"", "2024-09-22", "2023-11-05T01:00:00-08:00"}:           {},
	{"", "2024-09-22", "2023-12-31T23:59:59Z"}:                {},
	{"", "2024-09-22", "2024-01-01T00:00:00Z"}:                {},
	{"", "2024-09-22", "2024-02-29T12:00:00+05:30"}:           {},
	{"", "2024-09-22", "2024-02-29T12:00:00Z"}:                {},
	{"", "2024-09-22", "2024-03-10T02:00:00-08:00"}:           {},
	{"", "2024-09-22", "2024-06-15T13:45:30+09:00"}:           {},
	{"", "2024-09-22", "2024-07-04T23:59:59-07:00"}:           {},
	{"", "2024-09-22", "2024-08-30T18:30:00-04:00"}:           {},
	{"", "2024-09-22", "2024-09-21T00:00:00-03:00"}:           {},
	{"", "2024-10-31", "2022-01-01T00:00:00Z"}:                {},
	{"", "2024-10-31", "2023-02-28T23:59:59Z"}:                {},
	{"", "2024-10-31", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2024-10-31", "2023-07-19T14:30:00+09:00"}:           {},
	{"", "2024-10-31", "2023-07-19T14:30:00-07:00"}:           {},
	{"", "2024-10-31", "2023-07-19T14:30:00Z"}:                {},
	{"", "2024-10-31", "2023-11-05T01:00:00-08:00"}:           {},
	{"", "2024-10-31", "2023-12-31T23:59:59Z"}:                {},
	{"", "2024-10-31", "2024-01-01T00:00:00Z"}:                {},
	{"", "2024-10-31", "2024-02-29T12:00:00+05:30"}:           {},
	{"", "2024-10-31", "2024-02-29T12:00:00Z"}:                {},
	{"", "2024-10-31", "2024-03-10T02:00:00-08:00"}:           {},
	{"", "2024-10-31", "2024-06-15T13:45:30+09:00"}:           {},
	{"", "2024-10-31", "2024-07-04T23:59:59-07:00"}:           {},
	{"", "2024-10-31", "2024-08-30T18:30:00-04:00"}:           {},
	{"", "2024-10-31", "2024-09-21T00:00:00-03:00"}:           {},
	{"", "2024-10-31", "2024-10-31T17:00:00+00:00"}:           {},
	{"", "2024-11-05", "2022-01-01T00:00:00Z"}:                {},
	{"", "2024-11-05", "2023-02-28T23:59:59Z"}:                {},
	{"", "2024-11-05", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2024-11-05", "2023-07-19T14:30:00+09:00"}:           {},
	{"", "2024-11-05", "2023-07-19T14:30:00-07:00"}:           {},
	{"", "2024-11-05", "2023-07-19T14:30:00Z"}:                {},
	{"", "2024-11-05", "2023-11-05T01:00:00-08:00"}:           {},
	{"", "2024-11-05", "2023-12-31T23:59:59Z"}:                {},
	{"", "2024-11-05", "2024-01-01T00:00:00Z"}:                {},
	{"", "2024-11-05", "2024-02-29T12:00:00+05:30"}:           {},
	{"", "2024-11-05", "2024-02-29T12:00:00Z"}:                {},
	{"", "2024-11-05", "2024-03-10T02:00:00-08:00"}:           {},
	{"", "2024-11-05", "2024-06-15T13:45:30+09:00"}:           {},
	{"", "2024-11-05", "2024-07-04T23:59:59-07:00"}:           {},
	{"", "2024-11-05", "2024-08-30T18:30:00-04:00"}:           {},
	{"", "2024-11-05", "2024-09-21T00:00:00-03:00"}:           {},
	{"", "2024-11-05", "2024-10-31T17:00:00+00:00"}:           {},
	{"", "2024-11-05", "2024-11-05T08:00:00+01:00"}:           {},
	{"", "2024-12-26", "2022-01-01T00:00:00Z"}:                {},
	{"", "2024-12-26", "2023-02-28T23:59:59Z"}:                {},
	{"", "2024-12-26", "2023-03-12T02:00:00-07:00"}:           {},
	{"", "2024-12-26", "2023-07-19T14:30:00+09:00"}:           {},
	{"", "2024-12-26", "2023-07-19T14:30:00-07:00"}:           {},
	{"", "2024-12-26", "2023-07-19T14:30:00Z"}:                {},
	{"", "2024-12-26", "2023-11-05T01:00:00-08:00"}:           {},
	{"", "2024-12-26", "2023-12-31T23:59:59Z"}:                {},
	{"", "2024-12-26", "2024-01-01T00:00:00Z"}:                {},
	{"", "2024-12-26", "2024-02-29T12:00:00+05:30"}:           {},
	{"", "2024-12-26", "2024-02-29T12:00:00Z"}:                {},
	{"", "2024-12-26", "2024-03-10T02:00:00-08:00"}:           {},
	{"", "2024-12-26", "2024-06-15T13:45:30+09:00"}:           {},
	{"", "2024-12-26", "2024-07-04T23:59:59-07:00"}:           {},
	{"", "2024-12-26", "2024-08-30T18:30:00-04:00"}:           {},
	{"", "2024-12-26", "2024-09-21T00:00:00-03:00"}:           {},
	{"", "2024-12-26", "2024-10-31T17:00:00+00:00"}:           {},
	{"", "2024-12-26", "2024-11-05T08:00:00+01:00"}:           {},
	{"", "2024-12-26", "2024-12-25T00:00:00-05:00"}:           {},
	{"2016-12-31", "", "2022-01-01T00:00:00Z"}:                {},
	{"2016-12-31", "", "2023-02-28T23:59:59Z"}:                {},
	{"2016-12-31", "", "2023-03-12T02:00:00-07:00"}:           {},
	{"2016-12-31", "", "2023-07-19T14:30:00+09:00"}:           {},
	{"2016-12-31", "", "2023-07-19T14:30:00-07:00"}:           {},
	{"2016-12-31", "", "2023-07-19T14:30:00Z"}:                {},
	{"2016-12-31", "", "2023-11-05T01:00:00-08:00"}:           {},
	{"2016-12-31", "", "2023-12-31T23:59:59Z"}:                {},
	{"2016-12-31", "", "2024-01-01T00:00:00Z"}:                {},
	{"2016-12-31", "", "2024-02-29T12:00:00+05:30"}:           {},
	{"2016-12-31", "", "2024-02-29T12:00:00Z"}:                {},
	{"2016-12-31", "", "2024-03-10T02:00:00-08:00"}:           {},
	{"2016-12-31", "", "2024-06-15T13:45:30+09:00"}:           {},
	{"2016-12-31", "", "2024-07-04T23:59:59-07:00"}:           {},
	{"2016-12-31", "", "2024-08-30T18:30:00-04:00"}:           {},
	{"2016-12-31", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2016-12-31", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2016-12-31", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2016-12-31", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2022-01-01", "", "2022-01-01T00:00:00Z"}:                {},
	{"2022-01-01", "", "2023-02-28T23:59:59Z"}:                {},
	{"2022-01-01", "", "2023-03-12T02:00:00-07:00"}:           {},
	{"2022-01-01", "", "2023-07-19T14:30:00+09:00"}:           {},
	{"2022-01-01", "", "2023-07-19T14:30:00-07:00"}:           {},
	{"2022-01-01", "", "2023-07-19T14:30:00Z"}:                {},
	{"2022-01-01", "", "2023-11-05T01:00:00-08:00"}:           {},
	{"2022-01-01", "", "2023-12-31T23:59:59Z"}:                {},
	{"2022-01-01", "", "2024-01-01T00:00:00Z"}:                {},
	{"2022-01-01", "", "2024-02-29T12:00:00+05:30"}:           {},
	{"2022-01-01", "", "2024-02-29T12:00:00Z"}:                {},
	{"2022-01-01", "", "2024-03-10T02:00:00-08:00"}:           {},
	{"2022-01-01", "", "2024-06-15T13:45:30+09:00"}:           {},
	{"2022-01-01", "", "2024-07-04T23:59:59-07:00"}:           {},
	{"2022-01-01", "", "2024-08-30T18:30:00-04:00"}:           {},
	{"2022-01-01", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2022-01-01", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2022-01-01", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2022-01-01", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2022-01-01", "2022-01-31", "2022-01-01T00:00:00Z"}:      {},
	{"2023-02-01", "", "2023-02-28T23:59:59Z"}:                {},
	{"2023-02-01", "", "2023-03-12T02:00:00-07:00"}:           {},
	{"2023-02-01", "", "2023-07-19T14:30:00+09:00"}:           {},
	{"2023-02-01", "", "2023-07-19T14:30:00-07:00"}:           {},
	{"2023-02-01", "", "2023-07-19T14:30:00Z"}:                {},
	{"2023-02-01", "", "2023-11-05T01:00:00-08:00"}:           {},
	{"2023-02-01", "", "2023-12-31T23:59:59Z"}:                {},
	{"2023-02-01", "", "2024-01-01T00:00:00Z"}:                {},
	{"2023-02-01", "", "2024-02-29T12:00:00+05:30"}:           {},
	{"2023-02-01", "", "2024-02-29T12:00:00Z"}:                {},
	{"2023-02-01", "", "2024-03-10T02:00:00-08:00"}:           {},
	{"2023-02-01", "", "2024-06-15T13:45:30+09:00"}:           {},
	{"2023-02-01", "", "2024-07-04T23:59:59-07:00"}:           {},
	{"2023-02-01", "", "2024-08-30T18:30:00-04:00"}:           {},
	{"2023-02-01", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2023-02-01", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2023-02-01", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2023-02-01", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2023-02-01", "2023-02-28", "2023-02-28T23:59:59Z"}:      {},
	{"2023-03-10", "", "2023-03-12T02:00:00-07:00"}:           {},
	{"2023-03-10", "", "2023-07-19T14:30:00+09:00"}:           {},
	{"2023-03-10", "", "2023-07-19T14:30:00-07:00"}:           {},
	{"2023-03-10", "", "2023-07-19T14:30:00Z"}:                {},
	{"2023-03-10", "", "2023-11-05T01:00:00-08:00"}:           {},
	{"2023-03-10", "", "2023-12-31T23:59:59Z"}:                {},
	{"2023-03-10", "", "2024-01-01T00:00:00Z"}:                {},
	{"2023-03-10", "", "2024-02-29T12:00:00+05:30"}:           {},
	{"2023-03-10", "", "2024-02-29T12:00:00Z"}:                {},
	{"2023-03-10", "", "2024-03-10T02:00:00-08:00"}:           {},
	{"2023-03-10", "", "2024-06-15T13:45:30+09:00"}:           {},
	{"2023-03-10", "", "2024-07-04T23:59:59-07:00"}:           {},
	{"2023-03-10", "", "2024-08-30T18:30:00-04:00"}:           {},
	{"2023-03-10", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2023-03-10", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2023-03-10", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2023-03-10", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2023-03-10", "2023-03-12", "2023-03-12T02:00:00-07:00"}: {},
	{"2023-07-01", "", "2023-07-19T14:30:00+09:00"}:           {},
	{"2023-07-01", "", "2023-07-19T14:30:00-07:00"}:           {},
	{"2023-07-01", "", "2023-07-19T14:30:00Z"}:                {},
	{"2023-07-01", "", "2023-11-05T01:00:00-08:00"}:           {},
	{"2023-07-01", "", "2023-12-31T23:59:59Z"}:                {},
	{"2023-07-01", "", "2024-01-01T00:00:00Z"}:                {},
	{"2023-07-01", "", "2024-02-29T12:00:00+05:30"}:           {},
	{"2023-07-01", "", "2024-02-29T12:00:00Z"}:                {},
	{"2023-07-01", "", "2024-03-10T02:00:00-08:00"}:           {},
	{"2023-07-01", "", "2024-06-15T13:45:30+09:00"}:           {},
	{"2023-07-01", "", "2024-07-04T23:59:59-07:00"}:           {},
	{"2023-07-01", "", "2024-08-30T18:30:00-04:00"}:           {},
	{"2023-07-01", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2023-07-01", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2023-07-01", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2023-07-01", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2023-07-01", "2023-07-31", "2023-07-19T14:30:00+09:00"}: {},
	{"2023-07-01", "2023-07-31", "2023-07-19T14:30:00-07:00"}: {},
	{"2023-07-01", "2023-07-31", "2023-07-19T14:30:00Z"}:      {},
	{"2023-11-04", "", "2023-11-05T01:00:00-08:00"}:           {},
	{"2023-11-04", "", "2023-12-31T23:59:59Z"}:                {},
	{"2023-11-04", "", "2024-01-01T00:00:00Z"}:                {},
	{"2023-11-04", "", "2024-02-29T12:00:00+05:30"}:           {},
	{"2023-11-04", "", "2024-02-29T12:00:00Z"}:                {},
	{"2023-11-04", "", "2024-03-10T02:00:00-08:00"}:           {},
	{"2023-11-04", "", "2024-06-15T13:45:30+09:00"}:           {},
	{"2023-11-04", "", "2024-07-04T23:59:59-07:00"}:           {},
	{"2023-11-04", "", "2024-08-30T18:30:00-04:00"}:           {},
	{"2023-11-04", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2023-11-04", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2023-11-04", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2023-11-04", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2023-11-04", "2023-11-06", "2023-11-05T01:00:00-08:00"}: {},
	{"2023-12-01", "", "2023-12-31T23:59:59Z"}:                {},
	{"2023-12-01", "", "2024-01-01T00:00:00Z"}:                {},
	{"2023-12-01", "", "2024-02-29T12:00:00+05:30"}:           {},
	{"2023-12-01", "", "2024-02-29T12:00:00Z"}:                {},
	{"2023-12-01", "", "2024-03-10T02:00:00-08:00"}:           {},
	{"2023-12-01", "", "2024-06-15T13:45:30+09:00"}:           {},
	{"2023-12-01", "", "2024-07-04T23:59:59-07:00"}:           {},
	{"2023-12-01", "", "2024-08-30T18:30:00-04:00"}:           {},
	{"2023-12-01", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2023-12-01", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2023-12-01", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2023-12-01", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2023-12-01", "2023-12-31", "2023-12-31T23:59:59Z"}:      {},
	{"2024-01-01", "", "2024-01-01T00:00:00Z"}:                {},
	{"2024-01-01", "", "2024-02-29T12:00:00+05:30"}:           {},
	{"2024-01-01", "", "2024-02-29T12:00:00Z"}:                {},
	{"2024-01-01", "", "2024-03-10T02:00:00-08:00"}:           {},
	{"2024-01-01", "", "2024-06-15T13:45:30+09:00"}:           {},
	{"2024-01-01", "", "2024-07-04T23:59:59-07:00"}:           {},
	{"2024-01-01", "", "2024-08-30T18:30:00-04:00"}:           {},
	{"2024-01-01", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2024-01-01", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2024-01-01", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2024-01-01", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2024-01-01", "2024-01-31", "2024-01-01T00:00:00Z"}:      {},
	{"2024-02-01", "", "2024-02-29T12:00:00+05:30"}:           {},
	{"2024-02-01", "", "2024-02-29T12:00:00Z"}:                {},
	{"2024-02-01", "", "2024-03-10T02:00:00-08:00"}:           {},
	{"2024-02-01", "", "2024-06-15T13:45:30+09:00"}:           {},
	{"2024-02-01", "", "2024-07-04T23:59:59-07:00"}:           {},
	{"2024-02-01", "", "2024-08-30T18:30:00-04:00"}:           {},
	{"2024-02-01", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2024-02-01", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2024-02-01", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2024-02-01", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2024-02-01", "2024-02-29", "2024-02-29T12:00:00+05:30"}: {},
	{"2024-02-01", "2024-02-29", "2024-02-29T12:00:00Z"}:      {},
	{"2024-03-09", "", "2024-03-10T02:00:00-08:00"}:           {},
	{"2024-03-09", "", "2024-06-15T13:45:30+09:00"}:           {},
	{"2024-03-09", "", "2024-07-04T23:59:59-07:00"}:           {},
	{"2024-03-09", "", "2024-08-30T18:30:00-04:00"}:           {},
	{"2024-03-09", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2024-03-09", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2024-03-09", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2024-03-09", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2024-03-09", "2024-03-11", "2024-03-10T02:00:00-08:00"}: {},
	{"2024-06-01", "", "2024-06-15T13:45:30+09:00"}:           {},
	{"2024-06-01", "", "2024-07-04T23:59:59-07:00"}:           {},
	{"2024-06-01", "", "2024-08-30T18:30:00-04:00"}:           {},
	{"2024-06-01", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2024-06-01", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2024-06-01", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2024-06-01", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2024-06-01", "2024-06-15", "2024-06-15T13:45:30+09:00"}: {},
	{"2024-07-01", "", "2024-07-04T23:59:59-07:00"}:           {},
	{"2024-07-01", "", "2024-08-30T18:30:00-04:00"}:           {},
	{"2024-07-01", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2024-07-01", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2024-07-01", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2024-07-01", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2024-08-15", "", "2024-08-30T18:30:00-04:00"}:           {},
	{"2024-08-15", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2024-08-15", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2024-08-15", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2024-08-15", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2024-08-15", "2024-08-30", "2024-08-30T18:30:00-04:00"}: {},
	{"2024-09-20", "", "2024-09-21T00:00:00-03:00"}:           {},
	{"2024-09-20", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2024-09-20", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2024-09-20", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2024-09-20", "2024-09-22", "2024-09-21T00:00:00-03:00"}: {},
	{"2024-10-01", "", "2024-10-31T17:00:00+00:00"}:           {},
	{"2024-10-01", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2024-10-01", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2024-10-01", "2024-10-31", "2024-10-31T17:00:00+00:00"}: {},
	{"2024-11-01", "", "2024-11-05T08:00:00+01:00"}:           {},
	{"2024-11-01", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2024-11-01", "2024-11-05", "2024-11-05T08:00:00+01:00"}: {},
	{"2024-12-24", "", "2024-12-25T00:00:00-05:00"}:           {},
	{"2024-12-24", "2024-12-26", "2024-12-25T00:00:00-05:00"}: {},
}
