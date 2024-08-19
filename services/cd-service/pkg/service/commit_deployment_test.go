package service

import (
	"testing"
)

func TestGetCommitReleaseNumber(t *testing.T) {
	tcs := []struct {
		name      string
		eventJson []byte
		expected  uint64
	}{
		{
			name:      "ReleaseVersion doesn't exist in metadata",
			eventJson: []byte(`{"EventData":{"Environments":{"development":{},"staging":{}}},"EventMetadata":{"Uuid":"00000000-0000-0000-0000-000000000000","EventType":"new-release"}}`),
			expected:  0,
		},
		{
			name:      "ReleaseVersion exists in metadata",
			eventJson: []byte(`{"EventData":{"Environments":{"development":{},"staging":{}}},"EventMetadata":{"Uuid":"00000000-0000-0000-0000-000000000000","EventType":"new-release","ReleaseVersion":12}}`),
			expected:  12,
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			releaseVersion, err := getCommitReleaseNumber(tc.eventJson)
			if err != nil {
				t.Fatalf("Error getting release version: %v", err)
			}
			if releaseVersion != tc.expected {
				t.Fatalf("Expected %d, got %d", tc.expected, releaseVersion)
			}
		})
	}
}

func TestGetCommitEnvironments(t *testing.T) {
	tcs := []struct {
		name      string
		eventJson []byte
		expected  []string
	}{
		{
			name:      "One environment",
			eventJson: []byte(`{"EventData":{"Environments":{"development":{}}},"EventMetadata":{"Uuid":"00000000-0000-0000-0000-000000000000","EventType":"new-release"}}`),
			expected:  []string{"development"},
		},
		{
			name:      "Two environments",
			eventJson: []byte(`{"EventData":{"Environments":{"development":{},"staging":{}}},"EventMetadata":{"Uuid":"00000000-0000-0000-0000-000000000000","EventType":"new-release","ReleaseVersion":12}}`),
			expected:  []string{"development", "staging"},
		},
		{
			name:      "No environments",
			eventJson: []byte(`{"EventData":{},"EventMetadata":{"Uuid":"00000000-0000-0000-0000-000000000000","EventType":"new-release","ReleaseVersion":12}}`),
			expected:  []string{},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			environments, err := getCommitEnvironments(tc.eventJson)
			if err != nil {
				t.Fatalf("Error getting release version: %v", err)
			}
			if len(environments) != len(tc.expected) {
				t.Fatalf("Expected %d environments, got %d", len(tc.expected), len(environments))
			}
			for i, env := range environments {
				if env != tc.expected[i] {
					t.Fatalf("Expected %s, got %s", tc.expected[i], env)
				}
			}
		})
	}
}
