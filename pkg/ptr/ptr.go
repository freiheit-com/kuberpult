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

package ptr

func FromString(s string) *string {
	return &s
}

func ToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func Bool(b bool) *bool {
	return &b
}

func ToUint64(u *uint64) uint64 {
	if u == nil {
		return 0
	}
	return *u
}

func ToUint64Slice(us []int64) []uint64 {
	var result []uint64
	for i := range us {
		result = append(result, uint64(i))
	}
	return result
}
