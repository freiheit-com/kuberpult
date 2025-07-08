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
	"fmt"
	"github.com/hashicorp/go-version"
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

type ReleaseNumberCollection []ReleaseNumbers

func (c ReleaseNumberCollection) Less(i, j int) bool {
	fmt.Printf("I: %s\n", c[i].Revision)
	fmt.Printf("J: %s\n", c[j].Revision)
	v1, _ := version.NewVersion(c[i].Revision) //These should have already been validated
	v2, _ := version.NewVersion(c[j].Revision) //These should have already been validated
	return v1.LessThan(v2)
}

func (c ReleaseNumberCollection) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c ReleaseNumberCollection) Len() int {
	return len(c)
}
