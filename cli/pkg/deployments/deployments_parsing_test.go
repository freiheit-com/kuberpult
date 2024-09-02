/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/

package deployments

import (
	"fmt"
	"testing"
)

func TestValidateCommitHash(t *testing.T) {
	tests := []struct {
		name     string
		commit   string
		expected bool
	}{
		{
			name:     "length less than 40",
			commit:   "123456",
			expected: false,
		},
		{
			name:     "length more than 40",
			commit:   "12345678901234567890123456789012345678901",
			expected: false,
		},
		{
			name:     "invalid character",
			commit:   "1234567890123456789012345678901234567890g",
			expected: false,
		},
		{
			name:     "valid commit hash",
			commit:   "1234567890123456789012345678901234abcdef",
			expected: true,
		},
		{
			name:     "valid commit hash with uppercase letters",
			commit:   "1234567890123456789012345678901234ABCDEF",
			expected: true,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual := validateCommitHash(tc.commit)
			if actual != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestParseArgsCommitDeployments(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expected      *CommitDeploymentsParameters
		expecterError error
	}{
		{
			name: "valid commit hash",
			args: []string{"--commit", "1234567890123456789012345678901234abcdef"},
			expected: &CommitDeploymentsParameters{
				CommitId: "1234567890123456789012345678901234abcdef",
			},
			expecterError: nil,
		},
		{
			name:          "invalid commit hash",
			args:          []string{"--commit", "1234567890123456789012345678901234abcdeg"},
			expected:      nil,
			expecterError: fmt.Errorf("the commit hash: 1234567890123456789012345678901234abcdeg is invalid"),
		},
		{
			name:          "invalid number of arguments",
			args:          []string{"--commit", "1234567890123456789012345678901234abcdef", "extra"},
			expected:      nil,
			expecterError: fmt.Errorf("these arguments are not recognized: \"extra\""),
		},
		{
			name:          "missing commit flag",
			args:          []string{},
			expected:      nil,
			expecterError: fmt.Errorf("the commit hash must be set with the --commit flag"),
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cliArgs, err := ParseArgsCommitDeployments(tc.args)
			if err != nil {
				if tc.expecterError == nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if err.Error() != tc.expecterError.Error() {
					t.Fatalf("expected %v, got %v", tc.expecterError, err)
				}
				return
			}
			if cliArgs.CommitId != tc.expected.CommitId {
				t.Errorf("expected %v, got %v", tc.expected, cliArgs)
			}
			if cliArgs.OutFile != tc.expected.OutFile {
				t.Errorf("expected %v, got %v", tc.expected, cliArgs)
			}
		})
	}
}
