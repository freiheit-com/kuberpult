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
	"fmt"
	"strconv"
	"strings"
)

// a RepeatedString corresponds to a command line string argument that can be specified multiple times (possibly zero times)
// we further make the simplifying assumption that the string must be well-behaved
type RepeatedString struct {
	Values []string
}

func (rs *RepeatedString) Set(s string) error {
	rs.Values = append(rs.Values, s)
	return nil
}

func (rs *RepeatedString) String() string {
	return strings.Join(rs.Values, ",")
}

type RepeatedInt struct {
	Values []int64
}

func (rs *RepeatedInt) Set(s string) error {
	value, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("the provided value \"%s\" is not an integer", s)
	}
	rs.Values = append(rs.Values, value)
	return nil
}

func (rs *RepeatedInt) String() string {
	valuesAsStrings := make([]string, 0)
	for _, value := range rs.Values {
		valuesAsStrings = append(valuesAsStrings, fmt.Sprintf("%v", value))
	}
	return strings.Join(valuesAsStrings, ",")
}
