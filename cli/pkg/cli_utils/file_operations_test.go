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

package cli_utils

import (
	"os"
	"path"
	"testing"
)

func TestWriteToFile(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		content       []byte
		expectedError error
	}{
		{
			name:          "write to file",
			path:          "test.txt",
			content:       []byte("hello world"),
			expectedError: nil,
		},
	}
	for _, tc := range tests {
		tc := tc
		dir := t.TempDir()
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			outFile := path.Join(dir, tc.path)
			err := WriteToFile(outFile, tc.content)
			if err != tc.expectedError {
				t.Fatalf("expected %v, got %v", tc.expectedError, err)
			}
			content, err := os.ReadFile(outFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(content) != string(tc.content) {
				t.Errorf("expected %v, got %v", string(tc.content), string(content))
			}
		})
	}
}
