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

package types

import (
	"cmp"
	"fmt"
	"sort"
	"strings"
)

// EnvName is a type that helps us avoid mixing up envNames from other strings.
type EnvName string

func EnvNamesToStrings(a []EnvName) []string {
	var result = make([]string, len(a))
	for i := range a {
		result[i] = string(a[i])
	}
	return result
}

func StringsToEnvNames(a []string) []EnvName {
	var result = make([]EnvName, len(a))
	for i := range a {
		result[i] = EnvName(a[i])
	}
	return result
}

func Sort(a []EnvName) []EnvName {
	s := EnvNamesToStrings(a)
	sort.Strings(s)
	return StringsToEnvNames(s)
}

func StringPtr(a EnvName) *string {
	var result = string(a)
	return &result
}

func EnvNamePtr(a string) *EnvName {
	var result = EnvName(a)
	return &result
}

func EnvMapToStringMap[T comparable](a map[EnvName]T) map[string]T {
	var result = map[string]T{}
	for i := range a {
		result[string(i)] = a[i]
	}
	return result
}
func StringMapToEnvMap[T comparable](a map[string]T) map[EnvName]T {
	var result = map[EnvName]T{}
	for i := range a {
		result[EnvName(i)] = a[i]
	}
	return result
}

func Compare(a, b EnvName) int {
	return strings.Compare(string(a), string(b))
}

type ReleaseNumbers struct {
	Version  *uint64
	Revision uint64
}

func Greater(i, j ReleaseNumbers) bool {
	return CompareReleaseNumbers(i, j) > 0
}

func GreaterOrEqual(i, j ReleaseNumbers) bool {
	return CompareReleaseNumbers(i, j) >= 0
}

func CompareReleaseNumbers(a, b ReleaseNumbers) int {
	// Compare versions
	vCmp := cmp.Compare(*a.Version, *b.Version)
	if vCmp != 0 {
		return vCmp // Versions are different, return the result
	}
	// Versions are the same, compare revisions
	return cmp.Compare(a.Revision, b.Revision)
}

func (r ReleaseNumbers) String() string {
	if r.Version == nil {
		return "<nil_version>"
	}
	return fmt.Sprintf("%d.%d", *r.Version, r.Revision)
}

//func (r *ReleaseNumbers) String() string {
//	if r.Version == nil {
//		return "<nil_version>"
//	}
//	return fmt.Sprintf("%d.%d", *r.Version, r.Revision)
//}
//
//var _ fmt.Stringer = ReleaseNumbers{}         // Verify that T implements I.
//var _ fmt.Stringer = (*ReleaseNumbers{})(nil) // Verify that *T implements I.
