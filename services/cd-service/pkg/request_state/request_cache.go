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

package request_state

import (
	"context"
	"fmt"
)

type TeamNameByApp map[string]string

// RequestState contains all the data that we want to "cache" DURING ONE request.
// Do not use it to store anything that might CHANGE during one request.
// Do not use it to store anything OUTSIDE a request.
// We do not call it "cache", because we do not need to care about
// typical caching issues like proper invalidation.
type RequestState struct {
	// Everything in here should be private. We must always use a function to access the data, so the cache gets filled.
	teamNameByApp TeamNameByApp
}

func MakePrefilledRequestState(teamNameByApp TeamNameByApp) (*RequestState, error) {
	if teamNameByApp == nil {
		return nil, fmt.Errorf("requeststate: teamNameByApp nil")
	}
	return &RequestState{
		teamNameByApp: teamNameByApp,
	}, nil
}

func (c *RequestState) GetTeamNameByApp(_ context.Context, appName string) (string, error) {
	tmp, ok := c.teamNameByApp[appName]
	if ok {
		return tmp, nil
	}
	return "", fmt.Errorf("requeststate: no team name for app %s", appName)
}
