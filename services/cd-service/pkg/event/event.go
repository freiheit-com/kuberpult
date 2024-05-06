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
	"encoding/json"
	"errors"
	"fmt"
	"google.golang.org/protobuf/types/known/timestamppb"
	"io/fs"
	"slices"
	"time"

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
	Application                 string  `fs:"application" json:"Application"`
	Environment                 string  `fs:"environment" json:"Environment"`
	SourceTrainEnvironmentGroup *string `fs:"source_train_environment_group" json:"SourceTrainEnvironmentGroup"`
	SourceTrainUpstream         *string `fs:"source_train_upstream" json:"SourceTrainUpstream"`
}

func (_ *Deployment) eventType() string {
	return "deployment"
}

func (ev *Deployment) toProto(trg *api.Event) {
	var releaseTrainSource *api.DeploymentEvent_ReleaseTrainSource
	if ev.SourceTrainEnvironmentGroup != nil {
		releaseTrainSource = &api.DeploymentEvent_ReleaseTrainSource{
			UpstreamEnvironment:    "",
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

type LockPreventedDeployment struct {
	Application string `fs:"application"`
	Environment string `fs:"environment"`
	LockMessage string `fs:"lock_message"`
	LockType    string `fs:"lock_type"`
}

func (_ *LockPreventedDeployment) eventType() string {
	return "lock-prevented-deployment"
}

func (ev *LockPreventedDeployment) toProto(trg *api.Event) {
	var lockType api.LockPreventedDeploymentEvent_LockType
	switch ev.LockType {
	case "application":
		lockType = api.LockPreventedDeploymentEvent_LOCK_TYPE_APP
	case "environment":
		lockType = api.LockPreventedDeploymentEvent_LOCK_TYPE_ENV
	case "team":
		lockType = api.LockPreventedDeploymentEvent_LOCK_TYPE_TEAM
	default:
		lockType = api.LockPreventedDeploymentEvent_LOCK_TYPE_UNKNOWN
	}
	trg.EventType = &api.Event_LockPreventedDeploymentEvent{
		LockPreventedDeploymentEvent: &api.LockPreventedDeploymentEvent{
			Application: ev.Application,
			Environment: ev.Environment,
			LockMessage: ev.LockMessage,
			LockType:    lockType,
		},
	}
}

type ReplacedBy struct {
	Application       string `fs:"application"`
	Environment       string `fs:"environment"`
	CommitIDtoReplace string `fs:"commit"`
}

func (_ *ReplacedBy) eventType() string {
	return "replaced-by"
}

func (ev *ReplacedBy) toProto(trg *api.Event) {
	trg.EventType = &api.Event_ReplacedByEvent{
		ReplacedByEvent: &api.ReplacedByEvent{
			Application:        ev.Application,
			Environment:        ev.Environment,
			ReplacedByCommitId: ev.CommitIDtoReplace,
		},
	}
}

//type EventMetadata struct {
//	author string
//	UUID   strin
//}

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
		//exhaustruct:ignore
		result = &NewRelease{}
	case "deployment":
		//exhaustruct:ignore
		result = &Deployment{}
	case "lock-prevented-deployment":
		//exhaustruct:ignore
		result = &LockPreventedDeployment{}
	case "replaced-by":
		//exhaustruct:ignore
		result = &ReplacedBy{}
	default:
		return nil, fmt.Errorf("unknown event type: %q", tp.EventType)
	}
	if err := read(fs, eventDir, result); err != nil {
		return nil, err
	}
	return result, nil
}

func UnMarshallEvent(eventType string, eventJson string) (DBEventGo, error) {
	var concreteEvent Event
	var metadata Metadata
	var generalEvent EventJson

	err := json.Unmarshal([]byte(eventJson), &generalEvent)

	if err != nil {
		return DBEventGo{}, fmt.Errorf("Error processing general event. Json Unmarshall of general event failed: %s\n", eventJson)
	}

	switch eventType {
	case "new-release":
		//exhaustruct:ignore
		concreteEvent = &NewRelease{}
	case "deployment":
		concreteEvent = &Deployment{}
	case "lock-prevented-deployment":
		//exhaustruct:ignore
		concreteEvent = &LockPreventedDeployment{}
	case "replaced-by":
		//exhaustruct:ignore
		concreteEvent = &ReplacedBy{}
	default:
		return DBEventGo{}, fmt.Errorf("unknown event type: %q", eventType)
	}

	err = json.Unmarshal([]byte(generalEvent.DataJson), &concreteEvent)
	if err != nil {
		return DBEventGo{}, fmt.Errorf("Error processing event. Json Unmarshall of event of type '%s' failed: %s\n", eventType, generalEvent.DataJson)
	}

	err = json.Unmarshal([]byte(generalEvent.MetadataJson), &metadata)

	if err != nil {
		return DBEventGo{}, fmt.Errorf("Error processing event. Json Unmarshall of event of type '%s' failed: %s\n", eventType, generalEvent.DataJson)
	}
	return DBEventGo{
		EventMetadata: metadata,
		EventData:     concreteEvent,
	}, nil
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
		EventType: nil,
		CreatedAt: uuid.GetTime(&eventID),
		Uuid:      eventID.String(),
	}
	ev.toProto(result)
	return result
}

func DBToProto(ev Event, createdAt time.Time) *api.Event {
	result := &api.Event{
		EventType: nil,
		CreatedAt: timestamppb.New(createdAt),
		Uuid:      timeuuid.UUIDFromTime(createdAt).String(),
	}
	ev.toProto(result)
	return result
}

type EventJson struct {
	DataJson     string
	MetadataJson string
}

type Metadata struct {
	AuthorEmail string
}

type DBEventGo struct {
	EventData     Event
	EventMetadata Metadata
}
