package cmd

import (
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
)

func TestCalculateProcessDelay(t *testing.T) {
	exampleTime, err := time.Parse("2006-01-02 15:04:05", "2024-06-18 16:14:07")
	if err != nil {
		t.Fatal(err)
	}
	exampleTime10SecondsBefore := exampleTime.Add(-10 * time.Second)
	tcs := []struct {
		Name          string
		eslEvent      *db.EslEventRow
		currentTime   time.Time
		ExpectedDelay float64
	}{
		{
			Name:          "Should return 0 if there are no events",
			eslEvent:      nil,
			currentTime:   time.Now(),
			ExpectedDelay: 0,
		},
		{
			Name:          "Should return 0 if time created is not set",
			eslEvent:      &db.EslEventRow{},
			currentTime:   time.Now(),
			ExpectedDelay: 0,
		},
		{
			Name: "With one Event",
			eslEvent: &db.EslEventRow{
				EslId:     1,
				Created:   exampleTime10SecondsBefore,
				EventType: "CreateApplicationVersion",
				EventJson: "{}",
			},
			currentTime:   exampleTime,
			ExpectedDelay: 10,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := testutil.MakeTestContext()
			delay, err := calculateProcessDelay(ctx, tc.eslEvent, tc.currentTime)
			if err != nil {
				t.Fatal(err)
			}
			if delay != tc.ExpectedDelay {
				t.Errorf("expected %f, got %f", tc.ExpectedDelay, delay)
			}
		})
	}
}
