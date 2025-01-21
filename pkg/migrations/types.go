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

package migrations

import (
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"strconv"
	"strings"
)

func ParseKuberpultVersion(version string) (*api.KuberpultVersion, error) {
	splitDash := strings.Split(version, "-")
	if len(splitDash) == 4 {
		// we have a version in the form of pr-v11.6.10-7-g08f811e8
		// we need to find the second part in "pr-v11.6.10-7"
		// See the top-level Makefile and how the version is supplied to earthly in the line
		// "earthly -P +integration-test --kuberpult_version=$(IMAGE_TAG_KUBERPULT)"
		version = splitDash[1]
	} else if len(splitDash) == 3 {
		// this is what happens when we us run-kind.sh
		version = splitDash[0]
	} else if len(splitDash) == 1 {
		// this is the normal case, we don't have to remove parts
	} else {
		return nil, fmt.Errorf("invalid version, expected 0, 2, or 3 dashes, but got %s", version)
	}

	version = strings.TrimPrefix(version, "v")
	split := strings.Split(version, ".")
	if len(split) != 3 {
		return nil, fmt.Errorf("migration_cutoff: Error parsing kuberpult version '%s', must have 3 dots", version)
	}
	majorRaw := split[0]
	minorRaw := split[1]
	patchRaw := split[2]

	ma, err := strconv.ParseUint(majorRaw, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("migration_cutoff: Error parsing kuberpult major version'%s'. Error: %w", majorRaw, err)
	}
	mi, err := strconv.ParseUint(minorRaw, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("migration_cutoff: Error parsing kuberpult major version'%s'. Error: %w", majorRaw, err)
	}
	pa, err := strconv.ParseUint(patchRaw, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("migration_cutoff: Error parsing kuberpult major version'%s'. Error: %w", majorRaw, err)
	}
	return &api.KuberpultVersion{
		Major: int32(ma),
		Minor: int32(mi),
		Patch: int32(pa),
	}, nil
}

func CreateKuberpultVersion(major, minor, patch int) *api.KuberpultVersion {
	return &api.KuberpultVersion{
		Major: int32(major),
		Minor: int32(minor),
		Patch: int32(patch),
	}
}

func FormatKuberpultVersion(version *api.KuberpultVersion) string {
	if version == nil {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d", version.Major, version.Minor, version.Patch)
}

func IsKuberpultVersionEqual(a, b *api.KuberpultVersion) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Major == b.Major &&
		a.Minor == b.Minor &&
		a.Patch == b.Patch
}
