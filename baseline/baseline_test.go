package baseline

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Demonstrates how to calculate the range of UTC dates that overlap with a
// given timestamp range, where the timestamp range is defined as days before
// a given timestamp.
func ExampleWidenRange() {
	p := func(name, now string, days int) {
		t, _ := time.Parse(time.RFC3339, now)
		start, end := WidenRange(t.Add(oneDay*time.Duration(-days)), t)
		sd, ed := ExampleTimestampToDate(start, end)
		fmt.Printf("\n%s: %s, %d\n-> %s, %s\n-> %s, %s\n", name, now, days, start.Format(time.RFC3339), end.Format(time.RFC3339), sd, ed)
	}

	fmt.Println("---")
	p("utc-1", "2024-07-15T00:00:00Z", 1)
	p("utc-2", "2024-07-16T00:00:00Z", 1)

	fmt.Println("---")
	p("aest-1", "2024-07-15T00:00:00+10:00", 1)
	p("aest-2", "2024-07-16T00:00:00+10:00", 1)

	fmt.Println("---")
	p("morning-1", "2024-07-15T09:00:00+10:00", 1)
	p("morning-2", "2024-07-16T09:00:00+10:00", 1)

	fmt.Println("---")
	p("afternoon-1", "2024-07-15T15:00:00+10:00", 1)
	p("afternoon-2", "2024-07-16T15:00:00+10:00", 1)

	fmt.Println("---")
	p("afternoon3DayForNegativeOffset", "2024-07-16T15:00:00-11:35", 3)

	// output:
	//---
	//
	//utc-1: 2024-07-15T00:00:00Z, 1
	//-> 2024-07-14T00:00:00Z, 2024-07-15T00:00:00Z
	//-> 2024-07-14, 2024-07-14
	//
	//utc-2: 2024-07-16T00:00:00Z, 1
	//-> 2024-07-15T00:00:00Z, 2024-07-16T00:00:00Z
	//-> 2024-07-15, 2024-07-15
	//---
	//
	//aest-1: 2024-07-15T00:00:00+10:00, 1
	//-> 2024-07-13T10:00:00+10:00, 2024-07-15T10:00:00+10:00
	//-> 2024-07-13, 2024-07-14
	//
	//aest-2: 2024-07-16T00:00:00+10:00, 1
	//-> 2024-07-14T10:00:00+10:00, 2024-07-16T10:00:00+10:00
	//-> 2024-07-14, 2024-07-15
	//---
	//
	//morning-1: 2024-07-15T09:00:00+10:00, 1
	//-> 2024-07-13T10:00:00+10:00, 2024-07-15T10:00:00+10:00
	//-> 2024-07-13, 2024-07-14
	//
	//morning-2: 2024-07-16T09:00:00+10:00, 1
	//-> 2024-07-14T10:00:00+10:00, 2024-07-16T10:00:00+10:00
	//-> 2024-07-14, 2024-07-15
	//---
	//
	//afternoon-1: 2024-07-15T15:00:00+10:00, 1
	//-> 2024-07-14T10:00:00+10:00, 2024-07-16T10:00:00+10:00
	//-> 2024-07-14, 2024-07-15
	//
	//afternoon-2: 2024-07-16T15:00:00+10:00, 1
	//-> 2024-07-15T10:00:00+10:00, 2024-07-17T10:00:00+10:00
	//-> 2024-07-15, 2024-07-16
	//---
	//
	//afternoon3DayForNegativeOffset: 2024-07-16T15:00:00-11:35, 3
	//-> 2024-07-13T12:25:00-11:35, 2024-07-17T12:25:00-11:35
	//-> 2024-07-14, 2024-07-17
}

func ExampleExampleTimestampToDate() {
	p := func(name, startTime, endTime string) {
		start, _ := time.Parse(time.RFC3339, startTime)
		end, _ := time.Parse(time.RFC3339, endTime)
		startDate, endDate := ExampleTimestampToDate(start, end)
		start2, end2 := ExampleDateToTimestamp(startDate, endDate)
		fmt.Printf(
			"\n%s: [%s, %s)\n-> [%s, %s]\n-> [%s, %s)\n",
			name,
			startTime,
			endTime,
			startDate,
			endDate,
			start2.Format(time.RFC3339),
			end2.Format(time.RFC3339),
		)
	}

	fmt.Println("---")
	p("utc-1", "2024-07-15T00:00:00Z", "2024-07-16T00:00:00Z")
	p("utc-2", "2024-07-16T00:00:00Z", "2024-07-17T00:00:00Z")

	fmt.Println("---")
	p("aest-1", "2024-07-15T00:00:00+10:00", "2024-07-16T00:00:00+10:00")
	p("aest-2", "2024-07-16T00:00:00+10:00", "2024-07-17T00:00:00+10:00")

	fmt.Println("---")
	p("morning-1", "2024-07-15T09:00:00+10:00", "2024-07-16T09:00:00+10:00")
	p("morning-2", "2024-07-16T09:00:00+10:00", "2024-07-17T09:00:00+10:00")

	fmt.Println("---")
	p("afternoon-1", "2024-07-15T15:00:00+10:00", "2024-07-16T15:00:00+10:00")
	p("afternoon-2", "2024-07-16T15:00:00+10:00", "2024-07-17T15:00:00+10:00")

	fmt.Println("---")
	p("afternoon3DayForNegativeOffset", "2024-07-16T15:00:00-11:35", "2024-07-19T15:00:00-11:35")

	//output:
	//---
	//
	//utc-1: [2024-07-15T00:00:00Z, 2024-07-16T00:00:00Z)
	//-> [2024-07-15, 2024-07-15]
	//-> [2024-07-15T00:00:00Z, 2024-07-16T00:00:00Z)
	//
	//utc-2: [2024-07-16T00:00:00Z, 2024-07-17T00:00:00Z)
	//-> [2024-07-16, 2024-07-16]
	//-> [2024-07-16T00:00:00Z, 2024-07-17T00:00:00Z)
	//---
	//
	//aest-1: [2024-07-15T00:00:00+10:00, 2024-07-16T00:00:00+10:00)
	//-> [2024-07-15, 2024-07-14]
	//-> [2024-07-15T00:00:00Z, 2024-07-15T00:00:00Z)
	//
	//aest-2: [2024-07-16T00:00:00+10:00, 2024-07-17T00:00:00+10:00)
	//-> [2024-07-16, 2024-07-15]
	//-> [2024-07-16T00:00:00Z, 2024-07-16T00:00:00Z)
	//---
	//
	//morning-1: [2024-07-15T09:00:00+10:00, 2024-07-16T09:00:00+10:00)
	//-> [2024-07-15, 2024-07-14]
	//-> [2024-07-15T00:00:00Z, 2024-07-15T00:00:00Z)
	//
	//morning-2: [2024-07-16T09:00:00+10:00, 2024-07-17T09:00:00+10:00)
	//-> [2024-07-16, 2024-07-15]
	//-> [2024-07-16T00:00:00Z, 2024-07-16T00:00:00Z)
	//---
	//
	//afternoon-1: [2024-07-15T15:00:00+10:00, 2024-07-16T15:00:00+10:00)
	//-> [2024-07-16, 2024-07-15]
	//-> [2024-07-16T00:00:00Z, 2024-07-16T00:00:00Z)
	//
	//afternoon-2: [2024-07-16T15:00:00+10:00, 2024-07-17T15:00:00+10:00)
	//-> [2024-07-17, 2024-07-16]
	//-> [2024-07-17T00:00:00Z, 2024-07-17T00:00:00Z)
	//---
	//
	//afternoon3DayForNegativeOffset: [2024-07-16T15:00:00-11:35, 2024-07-19T15:00:00-11:35)
	//-> [2024-07-18, 2024-07-19]
	//-> [2024-07-18T00:00:00Z, 2024-07-20T00:00:00Z)
}

func TestExampleTimestampToDate(t *testing.T) {
	TestTimestampToDate(t, TimestampRangeValues, DateValues, ExampleMatches, ExampleTimestampToDate)
}

func TestTestTimestampToDateExternal_yoDawg(t *testing.T) {
	if err := TestTimestampToDateExternal(context.Background(), TimestampRangeValues, DateValues, ExampleMatches, ExampleTimestampToDate); err != nil {
		t.Fatal(err)
	}
}

func TestExampleDateToTimestamp(t *testing.T) {
	TestDateToTimestamp(t, DateRangeValues, TimestampValues, ExampleMatches, ExampleDateToTimestamp)
}

func FuzzExampleTimestampToDate(f *testing.F) {
	FuzzTimestampToDate(f, TimestampRangeValues, DateValues, ExampleTimestampToDate)
}
