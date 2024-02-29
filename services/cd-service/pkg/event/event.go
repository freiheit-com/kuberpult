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

Copyright 2023 freiheit.com*/
package event

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/uuid"
	"github.com/go-git/go-billy/v5"
	"github.com/onokonem/sillyQueueServer/timeuuid"
)

type eventType struct {
	EventType string `fs:"eventType"`
}

// NewRelease is an event that denotes that a commit has been released
// for the first time.
type NewRelease struct {
	Environments map[string]struct{} `fs:"environments"`
}

func (_ *NewRelease) eventType() string {
	return "new-release"
}

func (ev *NewRelease) toProto(trg *api.Event) {
	envs := make([]string, 0, len(ev.Environments))
	for env := range ev.Environments {
		envs = append(envs, env)
	}
	slices.Sort(envs)
	trg.EventType = &api.Event_CreateReleaseEvent{
		CreateReleaseEvent: &api.CreateReleaseEvent{
			EnvironmentNames: envs,
		},
	}
}

// Deployment is an event that denotes that an application of a commit
// has been released to an environment.
type Deployment struct {
	Application                 string  `fs:"application"`
	Environment                 string  `fs:"environment"`
	SourceTrainEnvironmentGroup *string `fs:"source_train_environment_group"`
	SourceTrainUpstream         *string `fs:"source_train_upstream"`
}

func (_ *Deployment) eventType() string {
	return "deployment"
}

func (ev *Deployment) toProto(trg *api.Event) {
	var releaseTrainSource *api.DeploymentEvent_ReleaseTrainSource
	if ev.SourceTrainEnvironmentGroup != nil {
		releaseTrainSource = &api.DeploymentEvent_ReleaseTrainSource{
			TargetEnvironmentGroup: ev.SourceTrainEnvironmentGroup,
		}
	}
	if ev.SourceTrainUpstream != nil {
		if releaseTrainSource == nil {
			releaseTrainSource = new(api.DeploymentEvent_ReleaseTrainSource)
		}
		releaseTrainSource.UpstreamEnvironment = *ev.SourceTrainUpstream
	}
	trg.EventType = &api.Event_DeploymentEvent{
		DeploymentEvent: &api.DeploymentEvent{
			Application:        ev.Application,
			TargetEnvironment:  ev.Environment,
			ReleaseTrainSource: releaseTrainSource,
		},
	}
}

// Event is a commit-releated event
type Event interface {
	eventType() string
	toProto(*api.Event)
}

// Read an event from a filesystem.
func Read(fs billy.Filesystem, eventDir string) (Event, error) {
	var tp eventType
	if err := read(fs, eventDir, &tp); err != nil {
		return nil, err
	}
	var result Event
	switch tp.EventType {
	case "new-release":
		result = &NewRelease{}
	case "deployment":
		result = &Deployment{}
	default:
		return nil, fmt.Errorf("unknown event type: %q", tp.EventType)
	}
	if err := read(fs, eventDir, result); err != nil {
		return nil, err
	}
	return result, nil
}

// Write an event to a filesystem
func Write(filesystem billy.Filesystem, eventDir string, event Event) error {
	_, err := filesystem.Stat(eventDir)
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("event file already exists: %w", err)
	}
	if err := write(filesystem, eventDir, eventType{
		EventType: event.eventType(),
	}); err != nil {
		return err
	}
	return write(filesystem, eventDir, event)
}

// Convert an event to its protobuf representation
func ToProto(eventID timeuuid.UUID, ev Event) *api.Event {
	result := &api.Event{
		CreatedAt: uuid.GetTime(&eventID),
		Uuid:      eventID.String(),
	}
	ev.toProto(result)
	return result
}
