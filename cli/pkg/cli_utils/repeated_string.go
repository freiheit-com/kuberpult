/*
This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright 2023 freiheit.com
*/

package cli_utils

import "regexp"
import "fmt"
import "strings"

// checks if a string is free of surprises.
// many Kuberpult endpoints don't mind special characters, this is meant to make the CLI simpler
func isWellBehavedString(s string) bool {
	match, err := regexp.MatchString("^[a-zA-Z0-9_\\./-]+$", s)
	return match && err == nil
}

// a RepeatedString corresponds to a command line string argument that can be specified multiple times (possibly zero times)
// we further make the simplifying assumption that the string must be well-behaved
type RepeatedString struct {
	Values []string
}

func (rs *RepeatedString) Set(s string) error {
	if !isWellBehavedString(s) {
		return fmt.Errorf("the string \"%s\" may not be used as a flag value, all values must match the regex ^[a-zA-Z0-9_\\./-]+$", s)
	}

	rs.Values = append(rs.Values, s)
	return nil
}

func (rs *RepeatedString) String() string {
	return strings.Join(rs.Values, ",")
}
