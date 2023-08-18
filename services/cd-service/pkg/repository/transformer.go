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

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"io"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/mapper"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/valid"

	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/grpc"

	"github.com/freiheit-com/kuberpult/pkg/auth"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"

	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
)

const (
	queueFileName = "queued_version"
	// number of old releases that will ALWAYS be kept in addition to the ones that are deployed:
	keptVersionsOnCleanup = 20
)

var (
	ErrReleaseAlreadyExist = fmt.Errorf("release already exists")
	ErrReleaseTooOld       = fmt.Errorf("release is too old")
)

func versionToString(Version uint64) string {
	return strconv.FormatUint(Version, 10)
}

func releasesDirectory(fs billy.Filesystem, application string) string {
	return fs.Join("applications", application, "releases")
}

func applicationDirectory(fs billy.Filesystem, application string) string {
	return fs.Join("applications", application)
}

func environmentDirectory(fs billy.Filesystem, environment string) string {
	return fs.Join("environments", environment)
}

func environmentApplicationDirectory(fs billy.Filesystem, environment, application string) string {
	return fs.Join("environments", environment, "applications", application)
}

func releasesDirectoryWithVersion(fs billy.Filesystem, application string, version uint64) string {
	return fs.Join(releasesDirectory(fs, application), versionToString(version))
}

func GetEnvironmentLocksCount(fs billy.Filesystem, env string) float64 {
	envLocksCount := 0
	envDir := environmentDirectory(fs, env)
	locksDir := fs.Join(envDir, "locks")
	if entries, _ := fs.ReadDir(locksDir); entries != nil {
		envLocksCount += len(entries)
	}
	return float64(envLocksCount)
}

func GetEnvironmentApplicationLocksCount(fs billy.Filesystem, environment, application string) float64 {
	envAppLocksCount := 0
	appDir := environmentApplicationDirectory(fs, environment, application)
	locksDir := fs.Join(appDir, "locks")
	if entries, _ := fs.ReadDir(locksDir); entries != nil {
		envAppLocksCount += len(entries)
	}
	return float64(envAppLocksCount)
}

func GaugeEnvLockMetric(fs billy.Filesystem, env string) {
	if ddMetrics != nil {
		ddMetrics.Gauge("env_lock_count", GetEnvironmentLocksCount(fs, env), []string{"env:" + env}, 1)
	}
}

func GaugeEnvAppLockMetric(fs billy.Filesystem, env, app string) {
	if ddMetrics != nil {
		ddMetrics.Gauge("app_lock_count", GetEnvironmentApplicationLocksCount(fs, env, app), []string{"app:" + app, "env:" + env}, 1)
	}
}

func UpdateDatadogMetrics(state *State) error {
	fs := state.Filesystem
	if ddMetrics == nil {
		return nil
	}
	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return err
	}
	for env := range configs {
		GaugeEnvLockMetric(fs, env)
		appsDir := fs.Join(environmentDirectory(fs, env), "applications")
		if entries, _ := fs.ReadDir(appsDir); entries != nil {
			for _, app := range entries {
				GaugeEnvAppLockMetric(fs, env, app.Name())
			}
		}
	}
	return nil
}

func RegularlySendDatadogMetrics(repo Repository, interval time.Duration, callBack func(repository Repository)) {
	metricEventTimer := time.NewTicker(interval * time.Second)
	for {
		select {
		case <-metricEventTimer.C:
			callBack(repo)
		}
	}
}

func GetRepositoryStateAndUpdateMetrics(repo Repository) {
	repoState := repo.State()
	if err := UpdateDatadogMetrics(repoState); err != nil {
		panic(err.Error())
	}
}

// A Transformer updates the files in the worktree
type Transformer interface {
	Transform(context.Context, *State) (commitMsg string, e error, changes *TransformerResult)
}

type TransformerFunc func(context.Context, *State) (string, error, *TransformerResult)

func (t TransformerFunc) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	return (t)(ctx, state)
}

type CreateApplicationVersion struct {
	Authentication
	Version        uint64
	Application    string
	Manifests      map[string]string
	SourceCommitId string
	SourceAuthor   string
	SourceMessage  string
	SourceRepoUrl  string
	Team           string
}

func GetLastRelease(fs billy.Filesystem, application string) (uint64, error) {
	var err error
	releasesDir := releasesDirectory(fs, application)
	err = fs.MkdirAll(releasesDir, 0777)
	if err != nil {
		return 0, err
	}
	if entries, err := fs.ReadDir(releasesDir); err != nil {
		return 0, err
	} else {
		var lastRelease uint64 = 0
		for _, e := range entries {
			if i, err := strconv.ParseUint(e.Name(), 10, 64); err != nil {
				//TODO(HVG): decide what to do with bad named releases
			} else {
				if i > lastRelease {
					lastRelease = i
				}
			}
		}
		return lastRelease, nil
	}
}

func (c *CreateApplicationVersion) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	fs := state.Filesystem
	version, err := c.calculateVersion(fs)
	if err != nil {
		return "", err, nil
	}
	if !valid.ApplicationName(c.Application) {
		return "", grpc.PublicError(ctx, fmt.Errorf("invalid application name: '%s' - must match regexp '%s' and <= %d characters", c.Application, valid.AppNameRegExp, valid.MaxAppNameLen)), nil
	}
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, version)
	appDir := applicationDirectory(fs, c.Application)
	if err = fs.MkdirAll(releaseDir, 0777); err != nil {
		return "", err, nil
	}

	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return "", err, nil
	}

	if c.SourceCommitId != "" {
		if err := util.WriteFile(fs, fs.Join(releaseDir, "source_commit_id"), []byte(c.SourceCommitId), 0666); err != nil {
			return "", err, nil
		}
	}
	if c.SourceAuthor != "" {
		if err := util.WriteFile(fs, fs.Join(releaseDir, "source_author"), []byte(c.SourceAuthor), 0666); err != nil {
			return "", err, nil
		}
	}
	if c.SourceMessage != "" {
		if err := util.WriteFile(fs, fs.Join(releaseDir, "source_message"), []byte(c.SourceMessage), 0666); err != nil {
			return "", err, nil
		}
	}
	if err := util.WriteFile(fs, fs.Join(releaseDir, "created_at"), []byte(getTimeNow(ctx).Format(time.RFC3339)), 0666); err != nil {
		return "", err, nil
	}
	if c.Team != "" {
		if err := util.WriteFile(fs, fs.Join(appDir, "team"), []byte(c.Team), 0666); err != nil {
			return "", err, nil
		}
	}
	if c.SourceRepoUrl != "" {
		if err := util.WriteFile(fs, fs.Join(appDir, "sourceRepoUrl"), []byte(c.SourceRepoUrl), 0666); err != nil {
			return "", err, nil
		}
	}
	result := ""
	isLatest, err := isLatestsVersion(state, c.Application, version)
	if err != nil {
		return "", err, nil
	}
	if !isLatest {
		// check that we can actually backfill this version
		oldVersions, err := findOldApplicationVersions(state, c.Application)
		if err != nil {
			return "", err, nil
		}
		for _, oldVersion := range oldVersions {
			if version == oldVersion {
				return "", ErrReleaseTooOld, nil
			}
		}
	}

	changes := &TransformerResult{}
	for env, man := range c.Manifests {
		err := state.checkUserPermissions(ctx, env, c.Application, auth.PermissionCreateRelease, c.RBACConfig)
		if err != nil {
			return "", err, nil
		}
		envDir := fs.Join(releaseDir, "environments", env)

		config, found := configs[env]
		hasUpstream := false
		if found {
			hasUpstream = config.Upstream != nil
		}

		if err = fs.MkdirAll(envDir, 0777); err != nil {
			return "", err, nil
		}
		if err := util.WriteFile(fs, fs.Join(envDir, "manifests.yaml"), []byte(man), 0666); err != nil {
			return "", err, nil
		}
		changes.AddAppEnv(c.Application, env)
		if hasUpstream && config.Upstream.Latest && isLatest {
			d := &DeployApplicationVersion{
				Environment:    env,
				Application:    c.Application,
				Version:        version, // the train should queue deployments, instead of giving up:
				LockBehaviour:  api.LockBehavior_Record,
				Authentication: c.Authentication,
			}
			deployResult, err, subChanges := d.Transform(ctx, state)
			changes.Combine(subChanges)
			if err != nil {
				_, ok := err.(*LockedError)
				if ok {
					continue // LockedErrors are expected
				} else {
					return "", err, nil
				}
			}
			result = result + deployResult + "\n"
		}
	}
	return fmt.Sprintf("created version %d of %q\n%s", version, c.Application, result), nil, changes
}

func (c *CreateApplicationVersion) calculateVersion(bfs billy.Filesystem) (uint64, error) {
	if c.Version == 0 {
		lastRelease, err := GetLastRelease(bfs, c.Application)
		if err != nil {
			return 0, err
		}
		return lastRelease + 1, nil
	} else {
		// check that the version doesn't already exist
		dir := releasesDirectoryWithVersion(bfs, c.Application, c.Version)
		_, err := bfs.Stat(dir)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return 0, err
			}
		} else {
			return 0, grpc.AlreadyExistsError(ErrReleaseAlreadyExist)
		}
		// TODO: check GC here
		return c.Version, nil
	}
}

func isLatestsVersion(state *State, application string, version uint64) (bool, error) {
	rels, err := state.GetApplicationReleases(application)
	if err != nil {
		return false, err
	}
	for _, r := range rels {
		if r > version {
			return false, nil
		}
	}
	return true, nil
}

type CreateUndeployApplicationVersion struct {
	Authentication
	Application string
}

func (c *CreateUndeployApplicationVersion) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	fs := state.Filesystem
	lastRelease, err := GetLastRelease(fs, c.Application)
	if err != nil {
		return "", err, nil
	}
	changes := &TransformerResult{}
	if lastRelease == 0 {
		return "", fmt.Errorf("cannot undeploy non-existing application '%v'", c.Application), nil
	}

	releaseDir := releasesDirectoryWithVersion(fs, c.Application, lastRelease+1)
	if err = fs.MkdirAll(releaseDir, 0777); err != nil {
		return "", err, nil
	}

	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return "", err, nil
	}
	// this is a flag to indicate that this is the special "undeploy" version
	if err := util.WriteFile(fs, fs.Join(releaseDir, "undeploy"), []byte(""), 0666); err != nil {
		return "", err, nil
	}
	if err := util.WriteFile(fs, fs.Join(releaseDir, "created_at"), []byte(getTimeNow(ctx).Format(time.RFC3339)), 0666); err != nil {
		return "", err, nil
	}
	result := ""
	for env := range configs {
		err := state.checkUserPermissions(ctx, env, c.Application, auth.PermissionCreateUndeploy, c.RBACConfig)
		if err != nil {
			return "", err, nil
		}
		envDir := fs.Join(releaseDir, "environments", env)

		config, found := configs[env]
		hasUpstream := false
		if found {
			hasUpstream = config.Upstream != nil
		}

		if err = fs.MkdirAll(envDir, 0777); err != nil {
			return "", err, nil
		}
		// note that the manifest is empty here!
		// but actually it's not quite empty!
		// The function we are using in DeployApplication version is `util.WriteFile`. And that does not allow overwriting files with empty content.
		// We work around this unusual behavior by writing a space into the file
		if err := util.WriteFile(fs, fs.Join(envDir, "manifests.yaml"), []byte(" "), 0666); err != nil {
			return "", err, nil
		}
		changes.AddAppEnv(c.Application, env)
		if hasUpstream && config.Upstream.Latest {
			d := &DeployApplicationVersion{
				Environment: env,
				Application: c.Application,
				Version:     lastRelease + 1,
				// the train should queue deployments, instead of giving up:
				LockBehaviour:  api.LockBehavior_Record,
				Authentication: c.Authentication,
			}
			deployResult, err, subChanges := d.Transform(ctx, state)
			changes.Combine(subChanges)
			if err != nil {
				_, ok := err.(*LockedError)
				if ok {
					continue // locked error are expected
				} else {
					return "", err, nil
				}
			}
			result = result + deployResult + "\n"
		}
	}
	return fmt.Sprintf("created undeploy-version %d of '%v'\n%s", lastRelease+1, c.Application, result), nil, changes
}

type UndeployApplication struct {
	Authentication
	Application string
}

func (u *UndeployApplication) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	fs := state.Filesystem
	lastRelease, err := GetLastRelease(fs, u.Application)
	if err != nil {
		return "", err, nil
	}
	if lastRelease == 0 {
		return "", fmt.Errorf("UndeployApplication: error cannot undeploy non-existing application '%v'", u.Application), nil
	}
	isUndeploy, err := state.IsUndeployVersion(u.Application, lastRelease)
	if err != nil {
		return "", err, nil
	}
	if !isUndeploy {
		return "", fmt.Errorf("UndeployApplication: error last release is not un-deployed application version of '%v'", u.Application), nil
	}
	appDir := applicationDirectory(fs, u.Application)
	configs, err := state.GetEnvironmentConfigs()
	for env := range configs {
		err := state.checkUserPermissions(ctx, env, u.Application, auth.PermissionDeployUndeploy, u.RBACConfig)
		if err != nil {
			return "", err, nil
		}
		envAppDir := environmentApplicationDirectory(fs, env, u.Application)
		entries, err := fs.ReadDir(envAppDir)
		if err != nil {
			return "", wrapFileError(err, envAppDir, "UndeployApplication: Could not open application directory. Does the app exist?"), nil
		}
		if entries == nil {
			// app was never deployed on this env, so we must ignore it!
			continue
		}

		appLocksDir := fs.Join(envAppDir, "locks")
		err = fs.Remove(appLocksDir)
		if err != nil {
			return "", fmt.Errorf("UndeployApplication: cannot delete app locks '%v'", appLocksDir), nil
		}

		versionDir := fs.Join(envAppDir, "version")
		undeployFile := fs.Join(versionDir, "undeploy")

		_, err = fs.Stat(versionDir)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			// if the app was never deployed here, that's not a reason to stop
			continue
		}

		_, err = fs.Stat(undeployFile)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("UndeployApplication: error cannot un-deploy application '%v' the release '%v' is not un-deployed: '%v'", u.Application, env, undeployFile), nil
		}

	}
	// remove application
	if err = fs.Remove(appDir); err != nil {
		return "", err, nil
	}
	for env := range configs {
		appDir := environmentApplicationDirectory(fs, env, u.Application)
		// remove environment application
		if err := fs.Remove(appDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("UndeployApplication: unexpected error application '%v' environment '%v': '%w'", u.Application, env, err), nil
		}
	}
	changes := &TransformerResult{} // TODO implement
	return fmt.Sprintf("application '%v' was deleted successfully", u.Application), nil, changes
}

type DeleteEnvFromApp struct {
	Authentication
	Application string
	Environment string
}

func (u *DeleteEnvFromApp) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	err := state.checkUserPermissions(ctx, u.Environment, u.Application, auth.PermissionDeleteEnvironmentApplication, u.RBACConfig)
	if err != nil {
		return "", err, nil
	}
	fs := state.Filesystem
	thisSprintf := func(format string, a ...any) string {
		return fmt.Sprintf("DeleteEnvFromApp app '%s' on env '%s': %s", u.Application, u.Environment, fmt.Sprintf(format, a...))
	}

	if u.Application == "" {
		return "", fmt.Errorf(thisSprintf("Need to provide the application")), nil
	}

	if u.Environment == "" {
		return "", fmt.Errorf(thisSprintf("Need to provide the environment")), nil
	}

	envAppDir := environmentApplicationDirectory(fs, u.Environment, u.Application)
	entries, err := fs.ReadDir(envAppDir)
	if err != nil {
		return "", wrapFileError(err, envAppDir, thisSprintf("Could not open application directory. Does the app exist?")), nil
	}

	if entries == nil {
		// app was never deployed on this env, so that's unusual - but for idempotency we treat it just like a success case:
		return fmt.Sprintf("Attempted to remove environment '%v' from application '%v' but it did not exist.", u.Environment, u.Application), nil, nil
	}

	err = fs.Remove(envAppDir)
	if err != nil {
		return "", wrapFileError(err, envAppDir, thisSprintf("Cannot delete app.'")), nil
	}

	changes := &TransformerResult{
		ChangedApps: []AppEnv{
			{
				App: u.Application,
				Env: u.Environment,
			},
		},
	}
	return fmt.Sprintf("Environment '%v' was removed from application '%v' successfully.", u.Environment, u.Application), nil, changes
}

type CleanupOldApplicationVersions struct {
	Application string
}

// Finds old releases for an application
func findOldApplicationVersions(state *State, name string) ([]uint64, error) {
	// 1) get release in each env:
	envConfigs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return nil, err
	}
	versions, err := state.GetApplicationReleases(name)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, err
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i] < versions[j]
	})
	// Use the latest version as oldest deployed version
	oldestDeployedVersion := versions[len(versions)-1]
	for env := range envConfigs {
		version, err := state.GetEnvironmentApplicationVersion(env, name)
		if err != nil {
			return nil, err
		}
		if version != nil {
			if *version < oldestDeployedVersion {
				oldestDeployedVersion = *version
			}
		}
	}
	positionOfOldestVersion := sort.Search(len(versions), func(i int) bool {
		return versions[i] >= oldestDeployedVersion
	})

	if positionOfOldestVersion < (keptVersionsOnCleanup - 1) {
		return nil, nil
	}
	return versions[0 : positionOfOldestVersion-(keptVersionsOnCleanup-1)], err
}

func (c *CleanupOldApplicationVersions) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	fs := state.Filesystem
	oldVersions, err := findOldApplicationVersions(state, c.Application)
	if err != nil {
		return "", fmt.Errorf("cleanup: could not get application releases for app '%s': %w", c.Application, err), nil
	}

	msg := ""
	for _, oldRelease := range oldVersions {
		// delete oldRelease:
		releasesDir := releasesDirectoryWithVersion(fs, c.Application, oldRelease)
		_, err := fs.Stat(releasesDir)
		if err != nil {
			return "", wrapFileError(err, releasesDir, "CleanupOldApplicationVersions: could not stat"), nil
		}
		err = fs.Remove(releasesDir)
		if err != nil {
			return "", fmt.Errorf("CleanupOldApplicationVersions: Unexpected error app %s: %w",
				c.Application, err), nil
		}
		msg = fmt.Sprintf("%sremoved version %d of app %v as cleanup\n", msg, oldRelease, c.Application)
	}
	changes := &TransformerResult{} // TODO implement
	return msg, nil, changes
}

func wrapFileError(e error, filename string, message string) error {
	return fmt.Errorf("%s '%s': %w", message, filename, e)
}

type Authentication struct {
	RBACConfig auth.RBACConfig
}

type CreateEnvironmentLock struct {
	Authentication
	Environment string
	LockId      string
	Message     string
}

func (s *State) checkUserPermissions(ctx context.Context, env, application, action string, RBACConfig auth.RBACConfig) error {
	if !RBACConfig.DexEnabled {
		return nil
	}
	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("checkUserPermissions: user not found: %v", err))
	}

	envs, err := s.GetEnvironmentConfigs()
	if err != nil {
		return err
	}
	var group string
	for envName, config := range envs {
		if envName == env {
			group = mapper.DeriveGroupName(config, env)
			break
		}
	}
	if group == "" {
		return fmt.Errorf("group not found for environment: %s", env)
	}
	return auth.CheckUserPermissions(RBACConfig, user, env, group, application, action)
}

// checkUserPermissionsCreateEnvironment check the permission for the environment creation action.
// This is a "special" case because the environment group is already provided on the request.
func (s *State) checkUserPermissionsCreateEnvironment(ctx context.Context, RBACConfig auth.RBACConfig, envConfig config.EnvironmentConfig) error {
	if !RBACConfig.DexEnabled {
		return nil
	}
	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("checkUserPermissions: user not found: %v", err))
	}
	envGroup := "*"
	// If an env group is provided on the request, use it on the permission.
	if envConfig.EnvironmentGroup != nil {
		envGroup = *(envConfig.EnvironmentGroup)
	}
	return auth.CheckUserPermissions(RBACConfig, user, "*", envGroup, "*", auth.PermissionCreateEnvironment)
}

func (c *CreateEnvironmentLock) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	err := state.checkUserPermissions(ctx, c.Environment, "*", auth.PermissionCreateLock, c.RBACConfig)
	if err != nil {
		return "", err, nil
	}
	fs := state.Filesystem
	envDir := fs.Join("environments", c.Environment)
	if _, err := fs.Stat(envDir); err != nil {
		return "", fmt.Errorf("error accessing dir %q: %w", envDir, err), nil
	} else {
		if chroot, err := fs.Chroot(envDir); err != nil {
			return "", err, nil
		} else {
			if err := createLock(ctx, chroot, c.LockId, c.Message); err != nil {
				return "", err, nil
			} else {
				GaugeEnvLockMetric(fs, c.Environment)
				changes := &TransformerResult{}
				return fmt.Sprintf("Created lock %q on environment %q", c.LockId, c.Environment), nil, changes
			}
		}
	}
}

func createLock(ctx context.Context, fs billy.Filesystem, lockId, message string) error {
	locksDir := "locks"
	if err := fs.MkdirAll(locksDir, 0777); err != nil {
		return err
	}

	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return err
	}

	// create lock dir
	newLockDir := fs.Join(locksDir, lockId)
	if err := fs.MkdirAll(newLockDir, 0777); err != nil {
		return err
	}

	// write message
	if err := util.WriteFile(fs, fs.Join(newLockDir, "message"), []byte(message), 0666); err != nil {
		return err
	}

	// write email
	if err := util.WriteFile(fs, fs.Join(newLockDir, "created_by_email"), []byte(user.Email), 0666); err != nil {
		return err
	}

	// write name
	if err := util.WriteFile(fs, fs.Join(newLockDir, "created_by_name"), []byte(user.Name), 0666); err != nil {
		return err
	}

	// write date in iso format
	if err := util.WriteFile(fs, fs.Join(newLockDir, "created_at"), []byte(getTimeNow(ctx).Format(time.RFC3339)), 0666); err != nil {
		return err
	}
	return nil
}

type DeleteEnvironmentLock struct {
	Authentication
	Environment string
	LockId      string
}

func (c *DeleteEnvironmentLock) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	err := state.checkUserPermissions(ctx, c.Environment, "*", auth.PermissionDeleteLock, c.RBACConfig)
	if err != nil {
		return "", err, nil
	}
	fs := state.Filesystem
	lockDir := fs.Join("environments", c.Environment, "locks", c.LockId)
	if err := fs.Remove(lockDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to delete directory %q: %w", lockDir, err), nil
	} else {
		s := State{
			Filesystem: fs,
		}
		apps, err := s.GetEnvironmentApplications(c.Environment)
		if err != nil {
			return "", fmt.Errorf("environment applications for %q not found: %v", c.Environment, err.Error()), nil
		}

		additionalMessageFromDeployment := ""
		for _, appName := range apps {
			queueMessage, err := s.ProcessQueue(ctx, fs, c.Environment, appName)
			if err != nil {
				return "", err, nil
			}
			if queueMessage != "" {
				additionalMessageFromDeployment = additionalMessageFromDeployment + "\n" + queueMessage
			}
		}
		GaugeEnvLockMetric(fs, c.Environment)
		changes := &TransformerResult{}
		return fmt.Sprintf("Deleted lock %q on environment %q%s", c.LockId, c.Environment, additionalMessageFromDeployment), nil, changes
	}
}

type CreateEnvironmentApplicationLock struct {
	Authentication
	Environment string
	Application string
	LockId      string
	Message     string
}

func (c *CreateEnvironmentApplicationLock) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	// Note: it's possible to lock an application BEFORE it's even deployed to the environment.
	err := state.checkUserPermissions(ctx, c.Environment, c.Application, auth.PermissionCreateLock, c.RBACConfig)
	if err != nil {
		return "", err, nil
	}
	fs := state.Filesystem
	envDir := fs.Join("environments", c.Environment)
	if _, err := fs.Stat(envDir); err != nil {
		return "", fmt.Errorf("error accessing dir %q: %w", envDir, err), nil
	} else {
		appDir := fs.Join(envDir, "applications", c.Application)
		if err := fs.MkdirAll(appDir, 0777); err != nil {
			return "", err, nil
		}
		if chroot, err := fs.Chroot(appDir); err != nil {
			return "", err, nil
		} else {
			if err := createLock(ctx, chroot, c.LockId, c.Message); err != nil {
				return "", err, nil
			} else {
				GaugeEnvAppLockMetric(fs, c.Environment, c.Application)
				changes := &TransformerResult{} // TODO implement
				return fmt.Sprintf("Created lock %q on environment %q for application %q", c.LockId, c.Environment, c.Application), nil, changes
			}
		}
	}
}

type DeleteEnvironmentApplicationLock struct {
	Authentication
	Environment string
	Application string
	LockId      string
}

func (c *DeleteEnvironmentApplicationLock) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	err := state.checkUserPermissions(ctx, c.Environment, c.Application, auth.PermissionDeleteLock, c.RBACConfig)
	if err != nil {
		return "", err, nil
	}
	fs := state.Filesystem
	lockDir := fs.Join("environments", c.Environment, "applications", c.Application, "locks", c.LockId)
	if err := fs.Remove(lockDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to delete directory %q: %w", lockDir, err), nil
	} else {
		s := State{
			Filesystem: fs,
		}
		queueMessage, err := s.ProcessQueue(ctx, fs, c.Environment, c.Application)
		if err != nil {
			return "", err, nil
		}
		GaugeEnvAppLockMetric(fs, c.Environment, c.Application)
		changes := &TransformerResult{}
		return fmt.Sprintf("Deleted lock %q on environment %q for application %q%s", c.LockId, c.Environment, c.Application, queueMessage), nil, changes
	}
}

type CreateEnvironment struct {
	Authentication
	Environment string
	Config      config.EnvironmentConfig
}

func (c *CreateEnvironment) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	err := state.checkUserPermissionsCreateEnvironment(ctx, c.RBACConfig, c.Config)
	if err != nil {
		return "", err, nil
	}
	fs := state.Filesystem
	envDir := fs.Join("environments", c.Environment)
	// Creation of environment is possible, but configuring it is not if running in bootstrap mode.
	// Configuration needs to be done by modifying config map in source repo
	if state.BootstrapMode && c.Config != (config.EnvironmentConfig{}) {
		return "", fmt.Errorf("Cannot create or update configuration in bootstrap mode. Please update configuration in config map instead."), nil
	}
	if err := fs.MkdirAll(envDir, 0777); err != nil {
		return "", err, nil
	} else {
		configFile := fs.Join(envDir, "config.json")
		file, err := fs.OpenFile(configFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
		if err != nil {
			return "", fmt.Errorf("error creating config: %w", err), nil
		}
		enc := json.NewEncoder(file)
		enc.SetIndent("", "  ")
		if err := enc.Encode(c.Config); err != nil {
			return "", fmt.Errorf("error writing json: %w", err), nil
		}
		changes := &TransformerResult{} // TODO implement
		return fmt.Sprintf("create environment %q", c.Environment), file.Close(), changes
	}
}

type QueueApplicationVersion struct {
	Environment string
	Application string
	Version     uint64
}

func (c *QueueApplicationVersion) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	fs := state.Filesystem
	// Create a symlink to the release
	applicationDir := fs.Join("environments", c.Environment, "applications", c.Application)
	if err := fs.MkdirAll(applicationDir, 0777); err != nil {
		return "", err, nil
	}
	queuedVersionFile := fs.Join(applicationDir, queueFileName)
	if err := fs.Remove(queuedVersionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err, nil
	}
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, c.Version)
	if err := fs.Symlink(fs.Join("..", "..", "..", "..", releaseDir), queuedVersionFile); err != nil {
		return "", err, nil
	}

	// TODO SU: maybe check here if that version is already deployed? or somewhere else ... or not at all...
	changes := &TransformerResult{}
	return fmt.Sprintf("Queued version %d of app %q in env %q", c.Version, c.Application, c.Environment), nil, changes
}

type DeployApplicationVersion struct {
	Authentication
	Environment   string
	Application   string
	Version       uint64
	LockBehaviour api.LockBehavior
}

func (c *DeployApplicationVersion) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	err := state.checkUserPermissions(ctx, c.Environment, c.Application, auth.PermissionDeployRelease, c.RBACConfig)
	if err != nil {
		return "", err, nil
	}
	fs := state.Filesystem
	// Check that the release exist and fetch manifest
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, c.Version)
	manifest := fs.Join(releaseDir, "environments", c.Environment, "manifests.yaml")
	manifestContent := []byte{}
	if file, err := fs.Open(manifest); err != nil {
		return "", wrapFileError(err, manifest, "could not open manifest"), nil
	} else {
		if content, err := io.ReadAll(file); err != nil {
			return "", err, nil
		} else {
			manifestContent = content
		}
		file.Close()
	}

	if c.LockBehaviour != api.LockBehavior_Ignore {
		// Check that the environment is not locked
		var (
			envLocks, appLocks map[string]Lock
			err                error
		)
		envLocks, err = state.GetEnvironmentLocks(c.Environment)
		if err != nil {
			return "", err, nil
		}
		appLocks, err = state.GetEnvironmentApplicationLocks(c.Environment, c.Application)
		if err != nil {
			return "", err, nil
		}
		if len(envLocks) > 0 || len(appLocks) > 0 {
			switch c.LockBehaviour {
			case api.LockBehavior_Record:
				q := QueueApplicationVersion{
					Environment: c.Environment,
					Application: c.Application,
					Version:     c.Version,
				}
				return q.Transform(ctx, state)
			case api.LockBehavior_Fail:
				return "", &LockedError{
					EnvironmentApplicationLocks: appLocks,
					EnvironmentLocks:            envLocks,
				}, nil
			case api.LockBehavior_Ignore:
				// just continue
			}
		}
	}
	// Create a symlink to the release
	applicationDir := fs.Join("environments", c.Environment, "applications", c.Application)
	if err := fs.MkdirAll(applicationDir, 0777); err != nil {
		return "", err, nil
	}
	versionFile := fs.Join(applicationDir, "version")
	if err := fs.Remove(versionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err, nil
	}
	if err := fs.Symlink(fs.Join("..", "..", "..", "..", releaseDir), versionFile); err != nil {
		return "", err, nil
	}
	// Copy the manifest for argocd
	manifestsDir := fs.Join(applicationDir, "manifests")
	if err := fs.MkdirAll(manifestsDir, 0777); err != nil {
		return "", err, nil
	}
	changes := &TransformerResult{}
	manifestFilename := fs.Join(manifestsDir, "manifests.yaml")
	// note that the manifest is empty here!
	// but actually it's not quite empty!
	// The function we are using here is `util.WriteFile`. And that does not allow overwriting files with empty content.
	// We work around this unusual behavior by writing a space into the file
	if len(manifestContent) == 0 {
		manifestContent = []byte(" ")
	}
	if err := util.WriteFile(fs, manifestFilename, manifestContent, 0666); err != nil {
		return "", err, nil
	}
	changes.AddAppEnv(c.Application, c.Environment)
	logger.FromContext(ctx).Warn(fmt.Sprintf("DeployApp: changes added: %v+", changes))

	user, err := auth.ReadUserFromContext(ctx)
	if err != nil {
		return "", err, nil
	}

	if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_by"), []byte(user.Name), 0666); err != nil {
		return "", err, nil
	}
	if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_by_email"), []byte(user.Email), 0666); err != nil {
		return "", err, nil
	}

	if err := util.WriteFile(fs, fs.Join(applicationDir, "deployed_at_utc"), []byte(getTimeNow(ctx).UTC().String()), 0666); err != nil {
		return "", err, nil
	}

	s := State{
		Filesystem: fs,
	}
	err = s.DeleteQueuedVersionIfExists(c.Environment, c.Application)
	if err != nil {
		return "", err, nil
	}
	d := &CleanupOldApplicationVersions{
		Application: c.Application,
	}
	transform, err, subChanges := d.Transform(ctx, state)
	logger.FromContext(ctx).Warn(fmt.Sprintf("DeployApp: sub changes: %v+", subChanges))
	changes.Combine(subChanges)
	if err != nil {
		return "", err, nil
	}

	logger.FromContext(ctx).Warn(fmt.Sprintf("DeployApp: combined changes: %v+", changes))
	return fmt.Sprintf("deployed version %d of %q to %q\n%s", c.Version, c.Application, c.Environment, transform), nil, changes
}

type ReleaseTrain struct {
	Authentication
	Target string
	Team   string
}

func getEnvironmentGroupsEnvironmentsOrEnvironment(configs map[string]config.EnvironmentConfig, targetGroupName string) map[string]config.EnvironmentConfig {

	envGroupConfigs := make(map[string]config.EnvironmentConfig)

	for env, config := range configs {
		if config.EnvironmentGroup != nil && *config.EnvironmentGroup == targetGroupName {
			envGroupConfigs[env] = config
		}
	}
	if len(envGroupConfigs) == 0 {
		envConfig, ok := configs[targetGroupName]
		if ok {
			envGroupConfigs[targetGroupName] = envConfig
		}
	}
	return envGroupConfigs
}

func generateReleaseTrainResponse(envDeployedMsg, envSkippedMsg map[string]string, targetGroupName string) string {
	resp := fmt.Sprintf("Release Train to environment/environment group '%s':\n\n", targetGroupName)

	// this to sort the env groups, to make sure that for the same input we always got the same output
	envGroups := make([]string, 0, len(envDeployedMsg))
	for env := range envDeployedMsg {
		envGroups = append(envGroups, env)
	}
	sort.Strings(envGroups)

	for _, env := range envGroups {
		msg := envDeployedMsg[env]
		resp += fmt.Sprintf("Release Train to '%s' environment:\n\n", env)
		resp += msg
		if skippedMsg, ok := envSkippedMsg[env]; ok {
			resp += "Skipped services:\n"
			resp += skippedMsg
		}
	}
	return resp
}

func (c *ReleaseTrain) Transform(ctx context.Context, state *State) (string, error, *TransformerResult) {
	var targetGroupName = c.Target

	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return "", grpc.InternalError(ctx, err), nil
	}
	var envGroupConfigs = getEnvironmentGroupsEnvironmentsOrEnvironment(configs, targetGroupName)

	if len(envGroupConfigs) == 0 {
		return "", grpc.PublicError(ctx, fmt.Errorf("could not find environment group or environment configs for '%v'", targetGroupName)), nil
	}

	// this to sort the env, to make sure that for the same input we always got the same output
	envGroups := make([]string, 0, len(envGroupConfigs))
	for env := range envGroupConfigs {
		envGroups = append(envGroups, env)
	}
	sort.Strings(envGroups)

	envDeployedMsg := make(map[string]string)
	envSkippedMsg := make(map[string]string)
	changes := &TransformerResult{}
	for _, envName := range envGroups {
		envConfig := envGroupConfigs[envName]
		if envConfig.Upstream == nil {
			envSkippedMsg[envName] = fmt.Sprintf("Environment %q does not have upstream configured - skipping.", envName)
			continue
		}
		err := state.checkUserPermissions(ctx, envName, "*", auth.PermissionDeployReleaseTrain, c.RBACConfig)
		if err != nil {
			return "", err, nil
		}

		var upstreamLatest = envConfig.Upstream.Latest
		var upstreamEnvName = envConfig.Upstream.Environment

		if !upstreamLatest && upstreamEnvName == "" {
			envSkippedMsg[envName] = fmt.Sprintf("Environment %q does not have upstream.latest or upstream.environment configured - skipping.", envName)
			continue
		}
		if upstreamLatest && upstreamEnvName != "" {
			envSkippedMsg[envName] = fmt.Sprintf("Environment %q has both upstream.latest and upstream.environment configured - skipping.", envName)
			continue
		}
		source := upstreamEnvName
		if upstreamLatest {
			source = "latest"
		}

		if !upstreamLatest {
			_, ok := configs[upstreamEnvName]
			if !ok {
				return fmt.Sprintf("Could not find environment config for upstream env %q. Target env was %q", upstreamEnvName, envName), err, nil
			}
		}

		envLocks, err := state.GetEnvironmentLocks(envName)
		if err != nil {
			return "", grpc.InternalError(ctx, fmt.Errorf("could not get lock for environment %q: %w", envName, err)), nil
		}
		if len(envLocks) > 0 {
			envSkippedMsg[envName] = fmt.Sprintf("Target Environment '%s' is locked - skipping.\n", envName)
			continue
		}

		var apps []string
		if upstreamLatest {
			apps, err = state.GetApplications()
			if err != nil {
				return "", grpc.InternalError(ctx, fmt.Errorf("could not get all applications for %q: %w", source, err)), nil
			}
		} else {
			apps, err = state.GetEnvironmentApplications(upstreamEnvName)
			if err != nil {
				return "", grpc.PublicError(ctx, fmt.Errorf("upstream environment (%q) does not have applications: %w", upstreamEnvName, err)), nil
			}
		}
		sort.Strings(apps)

		// now iterate over all apps, deploying all that are not locked
		numServices := 0
		completeMessage := ""
		for _, appName := range apps {
			if c.Team != "" {
				if team, err := state.GetApplicationTeamOwner(appName); err != nil {
					return "", nil, nil
				} else if c.Team != team {
					continue
				}
			}
			currentlyDeployedVersion, err := state.GetEnvironmentApplicationVersion(envName, appName)
			if err != nil {
				return "", grpc.PublicError(ctx, fmt.Errorf("application %q in env %q does not have a version deployed: %w", appName, envName, err)), nil
			}
			var versionToDeploy uint64
			if upstreamLatest {
				versionToDeploy, err = GetLastRelease(state.Filesystem, appName)
				if err != nil {
					return "", grpc.PublicError(ctx, fmt.Errorf("application %q does not have a latest deployed: %w", appName, err)), nil
				}
			} else {
				upstreamVersion, err := state.GetEnvironmentApplicationVersion(upstreamEnvName, appName)
				if err != nil {
					return "", grpc.PublicError(ctx, fmt.Errorf("application %q does not have a version deployed in env %q: %w", appName, upstreamEnvName, err)), nil
				}
				if upstreamVersion == nil {
					envSkippedMsg[envName] += fmt.Sprintf("skipping because there is no version for application %q in env %q \n", appName, upstreamEnvName)
					continue
				}
				versionToDeploy = *upstreamVersion
			}
			if currentlyDeployedVersion != nil && *currentlyDeployedVersion == versionToDeploy {
				envSkippedMsg[envName] += fmt.Sprintf("%sskipping %q because it is already in the version %d\n", completeMessage, appName, *currentlyDeployedVersion)
				continue
			}

			d := &DeployApplicationVersion{
				Environment:    envName, // here we deploy to the next env
				Application:    appName,
				Version:        versionToDeploy,
				LockBehaviour:  api.LockBehavior_Record,
				Authentication: c.Authentication,
			}
			transform, err, subChanges := d.Transform(ctx, state)
			if err != nil {
				_, ok := err.(*LockedError)
				if ok {
					continue // locked errors are to be expected
				}
				if errors.Is(err, os.ErrNotExist) {
					continue // some apps do not exist on all envs, we ignore those
				}
				return "", grpc.InternalError(ctx, fmt.Errorf("unexpected error while deploying app %q to env %q: %w", appName, envName, err)), nil
			}
			changes.Combine(subChanges)
			numServices += 1
			completeMessage = completeMessage + transform + "\n"
		}
		teamInfo := ""
		if c.Team != "" {
			teamInfo = " for team '" + c.Team + "'"
		}
		envDeployedMsg[envName] = fmt.Sprintf("The release train deployed %d services from '%s' to '%s'%s\n%s\n", numServices, source, envName, teamInfo, completeMessage)
	}

	return generateReleaseTrainResponse(envDeployedMsg, envSkippedMsg, targetGroupName), nil, changes
}
