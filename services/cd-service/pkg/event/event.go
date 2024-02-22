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
package event

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/go-git/go-billy/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type eventType struct {
	EventType string `fs:"eventType"`
}

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

type Deployment struct {
	AppLocks map[string]AppLock `fs:"appLocks"`
	EnvLocks map[string]EnvLock `fs:"envLocks"`
}

type AppLock struct {
	App     string `fs:"app"`
	Env     string `fs:"env"`
	Message string `fs:"message"`
}

type EnvLock struct {
	Env     string `fs:"env"`
	Message string `fs:"message"`
}

func (_ *Deployment) eventType() string {
	return "deployment"
}

func (ev *Deployment) toProto(trg *api.Event) {
	appLocks := make([]*api.DeploymentEvent_AppLock, len(ev.AppLocks))
	for lockID, lock := range ev.AppLocks {
		appLocks = append(appLocks, &api.DeploymentEvent_AppLock{
			Id:      lockID,
			App:     lock.App,
			Env:     lock.Env,
			Message: lock.Message,
		})
	}
	slices.SortFunc(appLocks, func(a, b *api.DeploymentEvent_AppLock) int {
		return strings.Compare(a.Id, b.Id)
	})
	envLocks := make([]*api.DeploymentEvent_EnvLock, len(ev.EnvLocks))
	for lockID, lock := range ev.EnvLocks {
		envLocks = append(envLocks, &api.DeploymentEvent_EnvLock{
			Id:      lockID,
			Env:     lock.Env,
			Message: lock.Message,
		})
	}
	slices.SortFunc(envLocks, func(a, b *api.DeploymentEvent_EnvLock) int {
		return strings.Compare(a.Id, b.Id)
	})
	trg.EventType = &api.Event_DeploymentEvent{
		DeploymentEvent: &api.DeploymentEvent{
			AppLocks: appLocks,
			EnvLocks: envLocks,
		},
	}
}

type Event interface {
	eventType() string
	toProto(*api.Event)
}

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

func ToProto(ev Event, createdAt *timestamppb.Timestamp) *api.Event {
	result := &api.Event{
		CreatedAt: createdAt,
	}
	ev.toProto(result)
	return result
}
