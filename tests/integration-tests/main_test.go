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
package integration_tests

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
)

// The appSuffix is a unique string consisting of only characters that are valid app names.
// This is used to make tests repeatable.
var appSuffix string

func base26(i uint64) string {
	result := ""
	if i == 0 {
		return "a"
	}
	for i > 0 {
		rem := i % 26
		i = i / 26
		result = string(rune('a'+rem)) + result
	}
	return result
}

// The app suffix is calculate by storing a counter in a file called .run-number.
func calculateAppSuffix() error {
	content, err := os.ReadFile(".run-number")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			content = []byte("0")
		} else {
			return err
		}
	}
	nr, err := strconv.ParseUint(string(content), 10, 64)
	if err != nil {
		return err
	}
	nextRun := nr + 1
	os.WriteFile(".run-number", []byte(fmt.Sprintf("%d", nextRun)), 0644)
	appSuffix = base26(nr)
	return nil
}

func TestMain(m *testing.M) {
	err := calculateAppSuffix()
	if err != nil {
		fmt.Printf("%s", err)
		os.Exit(2)
	}
	os.Exit(m.Run())
}
