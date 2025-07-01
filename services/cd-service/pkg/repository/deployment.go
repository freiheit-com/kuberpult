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

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/pkg/valid"
	"time"
)

func (c *DeployApplicationVersion) Transform(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
) (string, error) {
	prognosis, err := c.Prognosis(ctx, state, transaction)
	if err != nil {
		return "", err
	}
	return c.ApplyPrognosis(ctx, state, t, transaction, prognosis)
}

type DeployApplicationVersion struct {
	Authentication        `json:"-"`
	Environment           types.EnvName                   `json:"env"`
	Application           string                          `json:"app"`
	Version               uint64                          `json:"version"`
	LockBehaviour         api.LockBehavior                `json:"lockBehaviour"`
	WriteCommitData       bool                            `json:"writeCommitData"`
	SourceTrain           *DeployApplicationVersionSource `json:"sourceTrain"`
	Author                string                          `json:"author"`
	CiLink                string                          `json:"cilink"`
	TransformerEslVersion db.TransformerID                `json:"-"` // Tags the transformer with EventSourcingLight eslVersion
	SkipCleanup           bool                            `json:"-"`
}

func (c *DeployApplicationVersion) GetDBEventType() db.EventType {
	return db.EvtDeployApplicationVersion
}

func (c *DeployApplicationVersion) SetEslVersion(id db.TransformerID) {
	c.TransformerEslVersion = id
}

func (c *DeployApplicationVersion) GetEslVersion() db.TransformerID {
	return c.TransformerEslVersion
}

type DeployApplicationVersionSource struct {
	TargetGroup *string       `json:"targetGroup"`
	Upstream    types.EnvName `json:"upstream"`
}

type DeployPrognosis struct {
	TeamName          string
	EnvironmentConfig *config.EnvironmentConfig
	ManifestContent   []byte

	EnvLocks  map[string]Lock
	AppLocks  map[string]Lock
	TeamLocks map[string]Lock

	NewReleaseCommitId string
	ExistingDeployment *db.Deployment
	OldReleaseCommitId string
}

func (c *DeployApplicationVersion) Prognosis(
	ctx context.Context,
	state *State,
	transaction *sql.Tx,
) (*DeployPrognosis, error) {
	var manifestContent []byte
	version, err := state.DBHandler.DBSelectReleaseByVersion(ctx, transaction, c.Application, c.Version, true)
	if err != nil {
		return nil, err
	}
	if version == nil {
		return nil, fmt.Errorf("could not find version %d for app %s", c.Version, c.Application)
	}
	manifestContent = []byte(version.Manifests.Manifests[c.Environment])

	team, err := state.GetApplicationTeamOwner(ctx, transaction, c.Application)
	if err != nil {
		return nil, err
	}

	envConfig, err := state.GetEnvironmentConfigFromDB(ctx, transaction, c.Environment)
	if err != nil {
		return nil, err
	}

	err = state.checkUserPermissionsFromConfig(ctx, transaction, c.Environment, c.Application, auth.PermissionDeployRelease, team, c.RBACConfig, true, envConfig)
	if err != nil {
		return nil, err
	}

	var (
		envLocks, appLocks, teamLocks map[string]Lock // keys: lockId
	)
	envLocks, err = state.GetEnvironmentLocksFromDB(ctx, transaction, c.Environment)
	if err != nil {
		return nil, err
	}
	appLocks, err = state.GetEnvironmentApplicationLocks(ctx, transaction, c.Environment, c.Application)
	if err != nil {
		return nil, err
	}
	teamLocks, err = state.GetEnvironmentTeamLocks(ctx, transaction, c.Environment, team)
	if err != nil {
		return nil, err
	}

	newReleaseCommitId, _ := getCommitID(ctx, transaction, state, c.Version, c.Application)
	// continue anyway, it's ok if there is no commitId!

	existingDeployment, err := state.DBHandler.DBSelectLatestDeployment(ctx, transaction, c.Application, c.Environment)
	if err != nil {
		return nil, err
	}

	var existingVersion *uint64 = nil
	if existingDeployment != nil && existingDeployment.Version != nil {
		var tmp2 = (uint64)(*existingDeployment.Version)
		existingVersion = &tmp2
	}
	var oldReleaseCommitId = ""
	if existingVersion != nil {
		oldReleaseCommitId, _ = getCommitID(ctx, transaction, state, *existingVersion, c.Application)
		// continue anyway, this is only for events
	}

	return &DeployPrognosis{
		TeamName:           team,
		EnvironmentConfig:  envConfig,
		ManifestContent:    manifestContent,
		EnvLocks:           envLocks,
		AppLocks:           appLocks,
		TeamLocks:          teamLocks,
		NewReleaseCommitId: newReleaseCommitId,
		ExistingDeployment: existingDeployment,
		OldReleaseCommitId: oldReleaseCommitId,
	}, nil
}

func (c *DeployApplicationVersion) ApplyPrognosis(
	ctx context.Context,
	state *State,
	t TransformerContext,
	transaction *sql.Tx,
	prognosisData *DeployPrognosis,
) (string, error) {
	envName := c.Environment
	err := state.checkUserPermissionsFromConfig(ctx, transaction, envName, c.Application, auth.PermissionDeployRelease, prognosisData.TeamName, c.RBACConfig, true, prognosisData.EnvironmentConfig)
	if err != nil {
		return "", err
	}

	lockPreventedDeployment := false
	if c.LockBehaviour != api.LockBehavior_IGNORE {
		// Check that the environment is not locked
		if len(prognosisData.EnvLocks) > 0 || len(prognosisData.AppLocks) > 0 || len(prognosisData.TeamLocks) > 0 {
			if c.WriteCommitData {
				var lockType, lockMsg string
				if len(prognosisData.EnvLocks) > 0 {
					lockType = "environment"
					for _, lock := range prognosisData.EnvLocks {
						lockMsg = lock.Message
						break
					}
				} else {
					if len(prognosisData.AppLocks) > 0 {
						lockType = "application"
						for _, lock := range prognosisData.AppLocks {
							lockMsg = lock.Message
							break
						}
					} else {
						lockType = "team"
						for _, lock := range prognosisData.TeamLocks {
							lockMsg = lock.Message
							break
						}
					}
				}
				ev := createLockPreventedDeploymentEvent(c.Application, envName, lockMsg, lockType)
				if prognosisData.NewReleaseCommitId == "" {
					logger.FromContext(ctx).Sugar().Warnf("could not write event data - continuing. %v", fmt.Errorf("getCommitIDFromReleaseDir %v", err))
				} else {
					gen := getGenerator(ctx)
					eventUuid := gen.Generate()
					err = state.DBHandler.DBWriteLockPreventedDeploymentEvent(ctx, transaction, c.TransformerEslVersion, eventUuid, prognosisData.NewReleaseCommitId, ev)
					if err != nil {
						return "", GetCreateReleaseGeneralFailure(err)
					}
				}
				lockPreventedDeployment = true
			}
			switch c.LockBehaviour {
			case api.LockBehavior_RECORD:
				q := QueueApplicationVersion{
					Environment: c.Environment,
					Application: c.Application,
					Version:     c.Version,
				}
				return q.Transform(ctx, state, t, transaction)
			case api.LockBehavior_FAIL:
				return "", &LockedError{
					EnvironmentApplicationLocks: prognosisData.AppLocks,
					EnvironmentLocks:            prognosisData.EnvLocks,
					TeamLocks:                   prognosisData.TeamLocks,
				}
			}
		}
	}

	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return "", err
	}

	firstDeployment := false
	var oldVersion *int64

	if state.CloudRunClient != nil {
		err := state.CloudRunClient.DeployApplicationVersion(ctx, prognosisData.ManifestContent)
		if err != nil {
			return "", err
		}
	}
	if prognosisData.ExistingDeployment == nil || prognosisData.ExistingDeployment.Version == nil {
		firstDeployment = true
	} else {
		oldVersion = prognosisData.ExistingDeployment.Version
	}
	var v = int64(c.Version)
	newDeployment := db.Deployment{
		Created:       time.Time{},
		App:           c.Application,
		Env:           envName,
		Version:       &v,
		TransformerID: c.TransformerEslVersion,
		Metadata: db.DeploymentMetadata{
			DeployedByEmail: user.Email,
			DeployedByName:  user.Name,
			CiLink:          c.CiLink,
		},
	}
	err = state.DBHandler.DBUpdateOrCreateDeployment(ctx, transaction, newDeployment)
	if err != nil {
		return "", fmt.Errorf("could not write deployment for %v - %v", newDeployment, err)
	}
	t.AddAppEnv(c.Application, envName, prognosisData.TeamName)
	s := State{
		MinorRegexes:              state.MinorRegexes,
		MaxNumThreads:             state.MaxNumThreads,
		DBHandler:                 state.DBHandler,
		ReleaseVersionsLimit:      state.ReleaseVersionsLimit,
		CloudRunClient:            state.CloudRunClient,
		ParallelismOneTransaction: state.ParallelismOneTransaction,
	}
	err = s.DeleteQueuedVersionIfExists(ctx, transaction, envName, c.Application)
	if err != nil {
		return "", err
	}
	if !c.SkipCleanup {
		d := &CleanupOldApplicationVersions{
			Application:           c.Application,
			TransformerEslVersion: c.TransformerEslVersion,
		}

		if err := t.Execute(ctx, d, transaction); err != nil {
			return "", err
		}
	}
	if c.WriteCommitData { // write the corresponding event
		newReleaseCommitId := prognosisData.NewReleaseCommitId
		deploymentEvent := createDeploymentEvent(c.Application, c.Environment, c.SourceTrain)
		if newReleaseCommitId == "" {
			logger.FromContext(ctx).Sugar().Warnf("could not write event data - continuing. %v", fmt.Errorf("getCommitIDFromReleaseDir %v", err))
		} else {
			if !valid.SHA1CommitID(newReleaseCommitId) {
				logger.FromContext(ctx).Sugar().Warnf("skipping event because commit id was not found")
			} else {
				gen := getGenerator(ctx)
				eventUuid := gen.Generate()
				err = state.DBHandler.DBWriteDeploymentEvent(ctx, transaction, c.TransformerEslVersion, eventUuid, newReleaseCommitId, deploymentEvent)
				if err != nil {
					return "", GetCreateReleaseGeneralFailure(err)
				}
			}
		}

		if !firstDeployment && !lockPreventedDeployment {
			//If not first deployment and current deployment is successful, signal a new replaced by event
			if !valid.SHA1CommitID(newReleaseCommitId) {
				logger.FromContext(ctx).Sugar().Infof(
					"The source commit ID %s is not a valid/complete SHA1 hash, event cannot be stored.",
					newReleaseCommitId)
			} else {
				ev := createReplacedByEvent(c.Application, envName, newReleaseCommitId)
				if oldVersion == nil {
					logger.FromContext(ctx).Sugar().Errorf("did not find old version of app %s - skipping replaced-by event", c.Application)
				} else {
					gen := getGenerator(ctx)
					eventUuid := gen.Generate()
					v := uint64(*oldVersion)
					oldReleaseCommitId := prognosisData.OldReleaseCommitId
					if oldReleaseCommitId == "" {
						logger.FromContext(ctx).Sugar().Warnf("could not find commit for release %d of app %s - skipping replaced-by event", v, c.Application)
					} else {
						err = state.DBHandler.DBWriteReplacedByEvent(ctx, transaction, c.TransformerEslVersion, eventUuid, oldReleaseCommitId, ev)
						if err != nil {
							return "", err
						}
					}
				}
			}

		} else {
			logger.FromContext(ctx).Sugar().Infof(
				"Release to replace decteted, but could not retrieve new commit information. Replaced-by event not stored.")
		}
	}

	return fmt.Sprintf("deployed version %d of %q to %q", c.Version, c.Application, c.Environment), nil
}

func getCommitID(ctx context.Context, transaction *sql.Tx, state *State, release uint64, app string) (string, error) {
	tmp, err := state.DBHandler.DBSelectReleaseByVersion(ctx, transaction, app, release, true)
	if err != nil {
		return "", err
	}
	if tmp == nil {
		return "", fmt.Errorf("release %v not found for app %s", release, app)
	}
	if tmp.Metadata.SourceCommitId == "" {
		return "", fmt.Errorf("Found release %v for app %s, but commit id was empty", release, app)
	}
	return tmp.Metadata.SourceCommitId, nil
}

func createDeploymentEvent(application string, environment types.EnvName, sourceTrain *DeployApplicationVersionSource) *event.Deployment {
	ev := event.Deployment{
		SourceTrainEnvironmentGroup: nil,
		SourceTrainUpstream:         nil,
		Application:                 application,
		Environment:                 string(environment),
	}
	if sourceTrain != nil {
		if sourceTrain.TargetGroup != nil {
			ev.SourceTrainEnvironmentGroup = sourceTrain.TargetGroup
		}
		ev.SourceTrainUpstream = types.StringPtr(sourceTrain.Upstream)
	}
	return &ev
}

func createReplacedByEvent(application string, environment types.EnvName, commitId string) *event.ReplacedBy {
	ev := event.ReplacedBy{
		Application:       application,
		Environment:       string(environment),
		CommitIDtoReplace: commitId,
	}
	return &ev
}
