/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strconv"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/config"

	"github.com/go-git/go-billy/v5"
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

func UpdateDatadogMetrics(fs billy.Filesystem) error {
	if ddMetrics == nil {
		return nil
	}
	configs, err := (&State{Filesystem: fs}).GetEnvironmentConfigs()
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

// A Transformer updates the files in the worktree
type Transformer interface {
	Transform(context.Context, billy.Filesystem) (commitMsg string, e error)
}

type TransformerFunc func(context.Context, billy.Filesystem) (string, error)

func (t TransformerFunc) Transform(ctx context.Context, fs billy.Filesystem) (string, error) {
	return (t)(ctx, fs)
}

var _ Transformer = TransformerFunc(func(_ context.Context, _ billy.Filesystem) (string, error) { return "", nil })

type CreateApplicationVersion struct {
	Version        uint64
	Application    string
	Manifests      map[string]string
	SourceCommitId string
	SourceAuthor   string
	SourceMessage  string
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

func (c *CreateApplicationVersion) Transform(ctx context.Context, fs billy.Filesystem) (string, error) {
	version, err := c.calculateVersion(fs)
	if err != nil {
		return "", err
	}
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, version)
	appDir := applicationDirectory(fs, c.Application)
	if err = fs.MkdirAll(releaseDir, 0777); err != nil {
		return "", err
	}

	configs, err := (&State{Filesystem: fs}).GetEnvironmentConfigs()
	if err != nil {
		return "", err
	}

	if c.SourceCommitId != "" {
		if err := util.WriteFile(fs, fs.Join(releaseDir, "source_commit_id"), []byte(c.SourceCommitId), 0666); err != nil {
			return "", err
		}
	}
	if c.SourceAuthor != "" {
		if err := util.WriteFile(fs, fs.Join(releaseDir, "source_author"), []byte(c.SourceAuthor), 0666); err != nil {
			return "", err
		}
	}
	if c.SourceMessage != "" {
		if err := util.WriteFile(fs, fs.Join(releaseDir, "source_message"), []byte(c.SourceMessage), 0666); err != nil {
			return "", err
		}
	}
	if c.Team != "" {
		if err := util.WriteFile(fs, fs.Join(appDir, "team"), []byte(c.Team), 0666); err != nil {
			return "", err
		}
	}
	result := ""
	isLatest, err := isLatestsVersion(fs, c.Application, version)
	if err != nil {
		return "", err
	}
	if isLatest {
		for env, man := range c.Manifests {
			envDir := fs.Join(releaseDir, "environments", env)

			config, found := configs[env]
			hasUpstream := false
			if found {
				hasUpstream = config.Upstream != nil
			}

			if err = fs.MkdirAll(envDir, 0777); err != nil {
				return "", err
			}
			if err := util.WriteFile(fs, fs.Join(envDir, "manifests.yaml"), []byte(man), 0666); err != nil {
				return "", err
			}

			if hasUpstream && config.Upstream.Latest {
				d := &DeployApplicationVersion{
					Environment:   env,
					Application:   c.Application,
					Version:       version, // the train should queue deployments, instead of giving up:
					LockBehaviour: api.LockBehavior_Queue,
				}
				deployResult, err := d.Transform(ctx, fs)
				if err != nil {
					_, ok := err.(*LockedError)
					if ok {
						continue // locked error are expected
					} else {
						return "", err
					}
				}
				result = result + deployResult + "\n"
			}
		}
	} else {
		// check that we can actually backfill this version
		oldVersions, err := findOldApplicationVersions(fs, c.Application)
		if err != nil {
			return "", err
		}
		for _, oldVersion := range oldVersions {
			if version == oldVersion {
				return "", ErrReleaseTooOld
			}
		}

	}
	return fmt.Sprintf("created version %d of %q\n%s", version, c.Application, result), nil
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
			return 0, ErrReleaseAlreadyExist
		}
		// TODO: check GC here
		return c.Version, nil
	}
}

func isLatestsVersion(bfs billy.Filesystem, application string, version uint64) (bool, error) {
	rels, err := (&State{Filesystem: bfs}).GetApplicationReleases(application)
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
	Application string
}

func (c *CreateUndeployApplicationVersion) Transform(ctx context.Context, fs billy.Filesystem) (string, error) {
	lastRelease, err := GetLastRelease(fs, c.Application)
	if err != nil {
		return "", err
	}
	if lastRelease == 0 {
		return "", fmt.Errorf("cannot undeploy non-existing application '%v'", c.Application)
	}

	releaseDir := releasesDirectoryWithVersion(fs, c.Application, lastRelease+1)
	if err = fs.MkdirAll(releaseDir, 0777); err != nil {
		return "", err
	}

	configs, err := (&State{Filesystem: fs}).GetEnvironmentConfigs()
	if err != nil {
		return "", err
	}
	// this is a flag to indicate that this is the special "undeploy" version
	if err := util.WriteFile(fs, fs.Join(releaseDir, "undeploy"), []byte(""), 0666); err != nil {
		return "", err
	}
	result := ""
	for env := range configs {
		envDir := fs.Join(releaseDir, "environments", env)

		config, found := configs[env]
		hasUpstream := false
		if found {
			hasUpstream = config.Upstream != nil
		}

		if err = fs.MkdirAll(envDir, 0777); err != nil {
			return "", err
		}
		// note that the manifest is empty here!
		if err := util.WriteFile(fs, fs.Join(envDir, "manifests.yaml"), []byte(""), 0666); err != nil {
			return "", err
		}

		if hasUpstream && config.Upstream.Latest {
			d := &DeployApplicationVersion{
				Environment: env,
				Application: c.Application,
				Version:     lastRelease + 1,
				// the train should queue deployments, instead of giving up:
				LockBehaviour: api.LockBehavior_Queue,
			}
			deployResult, err := d.Transform(ctx, fs)
			if err != nil {
				_, ok := err.(*LockedError)
				if ok {
					continue // locked error are expected
				} else {
					return "", err
				}
			}
			result = result + deployResult + "\n"
		}
	}
	return fmt.Sprintf("created undeploy-version %d of '%v'\n%s", lastRelease+1, c.Application, result), nil
}

type UndeployApplication struct {
	Application string
}

func (u *UndeployApplication) Transform(ctx context.Context, fs billy.Filesystem) (string, error) {
	lastRelease, err := GetLastRelease(fs, u.Application)
	if err != nil {
		return "", err
	}
	if lastRelease == 0 {
		return "", fmt.Errorf("UndeployApplication: error cannot undeploy non-existing application '%v'", u.Application)
	}
	isUndeploy, err := (&State{Filesystem: fs}).IsUndeployVersion(u.Application, lastRelease)
	if err != nil {
		return "", err
	}
	if !isUndeploy {
		return "", fmt.Errorf("UndeployApplication: error last release is not un-deployed application version of '%v'", u.Application)
	}
	appDir := applicationDirectory(fs, u.Application)
	configs, err := (&State{Filesystem: fs}).GetEnvironmentConfigs()
	for env := range configs {
		envAppDir := environmentApplicationDirectory(fs, env, u.Application)
		locksDir := fs.Join(envAppDir, "locks")
		undeployFile := fs.Join(envAppDir, "version", "undeploy")

		if entries, _ := fs.ReadDir(locksDir); entries != nil {
			return "", fmt.Errorf("UndeployApplication: error cannot un-deploy application '%v' unlock the application lock in the '%v' environment first", u.Application, env)
		}

		if _, err := fs.Stat(undeployFile); err != nil && errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("UndeployApplication: error cannot un-deploy application '%v' the release '%v' is not un-deployed", u.Application, env)
		}
	}
	// remove application
	if err = fs.Remove(appDir); err != nil {
		return "", err
	}
	for env := range configs {
		appDir := environmentApplicationDirectory(fs, env, u.Application)
		// remove environment application
		if err := fs.Remove(appDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("UndeployApplication: unexpected error application '%v' environment '%v': '%w'", u.Application, env, err)
		}
	}
	return fmt.Sprintf("application '%v' was deleted successfully", u.Application), nil
}

type CleanupOldApplicationVersions struct {
	Application string
}

// Finds old releases for an application
func findOldApplicationVersions(fs billy.Filesystem, name string) ([]uint64, error) {
	var state = &State{Filesystem: fs}
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

func (c *CleanupOldApplicationVersions) Transform(ctx context.Context, fs billy.Filesystem) (string, error) {
	oldVersions, err := findOldApplicationVersions(fs, c.Application)
	if err != nil {
		return "", fmt.Errorf("cleanup: could not get application releases for app '%s': %w", c.Application, err)
	}

	msg := ""
	for _, oldRelease := range oldVersions {
		// delete oldRelease:
		releasesDir := releasesDirectoryWithVersion(fs, c.Application, oldRelease)
		_, err := fs.Stat(releasesDir)
		if err != nil {
			return "", wrapFileError(err, releasesDir, "CleanupOldApplicationVersions: could not stat")
		}
		err = fs.Remove(releasesDir)
		if err != nil {
			return "", fmt.Errorf("CleanupOldApplicationVersions: Unexpected error app %s: %w",
				c.Application, err)
		}
		msg = fmt.Sprintf("%sremoved version %d of app %v as cleanup\n", msg, oldRelease, c.Application)
	}
	return msg, nil
}

func wrapFileError(e error, filename string, message string) error {
	return fmt.Errorf("%s '%s': %w", message, filename, e)
}

type CreateEnvironmentLock struct {
	Environment string
	LockId      string
	Message     string
}

func (c *CreateEnvironmentLock) Transform(ctx context.Context, fs billy.Filesystem) (string, error) {
	envDir := fs.Join("environments", c.Environment)
	if _, err := fs.Stat(envDir); err != nil {
		return "", fmt.Errorf("error accessing dir %q: %w", envDir, err)
	} else {
		if chroot, err := fs.Chroot(envDir); err != nil {
			return "", err
		} else {
			if err := createLock(chroot, c.LockId, c.Message); err != nil {
				return "", err
			} else {
				GaugeEnvLockMetric(fs, c.Environment)
				return fmt.Sprintf("created lock %q on environment %q", c.LockId, c.Environment), nil
			}
		}
	}
}

func createLock(fs billy.Filesystem, lockId, message string) error {
	locksDir := "locks"
	if err := fs.MkdirAll(locksDir, 0777); err != nil {
		return err
	}
	locksFile := fs.Join(locksDir, lockId)
	if err := util.WriteFile(fs, locksFile, []byte(message), 0666); err != nil {
		return err
	}
	return nil
}

type DeleteEnvironmentLock struct {
	Environment string
	LockId      string
}

func (c *DeleteEnvironmentLock) Transform(ctx context.Context, fs billy.Filesystem) (string, error) {

	file := fs.Join("environments", c.Environment, "locks", c.LockId)
	if err := fs.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	} else {
		s := State{
			Filesystem: fs,
		}
		apps, err := s.GetEnvironmentApplications(c.Environment)
		if err != nil {
			return "", fmt.Errorf("environment applications for %q not found: %v", c.Environment, err.Error())
		}

		additionalMessageFromDeployment := ""
		for _, appName := range apps {
			queueMessage, err := s.ProcessQueue(ctx, fs, c.Environment, appName)
			if err != nil {
				return "", err
			}
			if queueMessage != "" {
				additionalMessageFromDeployment = additionalMessageFromDeployment + "\n" + queueMessage
			}
		}
		GaugeEnvLockMetric(fs, c.Environment)
		return fmt.Sprintf("unlocked environment %q%s", c.Environment, additionalMessageFromDeployment), nil
	}
}

type CreateEnvironmentApplicationLock struct {
	Environment string
	Application string
	LockId      string
	Message     string
}

func (c *CreateEnvironmentApplicationLock) Transform(ctx context.Context, fs billy.Filesystem) (string, error) {
	// Note: it's possible to lock an application BEFORE it's even deployed to the environment.
	envDir := fs.Join("environments", c.Environment)
	if _, err := fs.Stat(envDir); err != nil {
		return "", fmt.Errorf("error accessing dir %q: %w", envDir, err)
	} else {
		appDir := fs.Join(envDir, "applications", c.Application)
		if err := fs.MkdirAll(appDir, 0777); err != nil {
			return "", err
		}
		if chroot, err := fs.Chroot(appDir); err != nil {
			return "", err
		} else {
			if err := createLock(chroot, c.LockId, c.Message); err != nil {
				return "", err
			} else {
				GaugeEnvAppLockMetric(fs, c.Environment, c.Application)
				return fmt.Sprintf("created lock %q on environment %q for application %q", c.LockId, c.Environment, c.Application), nil
			}
		}
	}
}

type DeleteEnvironmentApplicationLock struct {
	Environment string
	Application string
	LockId      string
}

func (c *DeleteEnvironmentApplicationLock) Transform(ctx context.Context, fs billy.Filesystem) (string, error) {
	file := fs.Join("environments", c.Environment, "applications", c.Application, "locks", c.LockId)
	if err := fs.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	} else {
		s := State{
			Filesystem: fs,
		}
		queueMessage, err := s.ProcessQueue(ctx, fs, c.Environment, c.Application)
		if err != nil {
			return "", err
		}
		GaugeEnvAppLockMetric(fs, c.Environment, c.Application)
		return fmt.Sprintf("unlocked application %q in environment %q%q", c.Application, c.Environment, queueMessage), nil
	}
}

type CreateEnvironment struct {
	Environment string
	Config      config.EnvironmentConfig
}

func (c *CreateEnvironment) Transform(ctx context.Context, fs billy.Filesystem) (string, error) {
	envDir := fs.Join("environments", c.Environment)
	if err := fs.MkdirAll(envDir, 0777); err != nil {
		return "", err
	} else {
		configFile := fs.Join(envDir, "config.json")
		file, err := fs.OpenFile(configFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
		if err != nil {
			return "", fmt.Errorf("error creating config: %w", err)
		}
		enc := json.NewEncoder(file)
		if err := enc.Encode(c.Config); err != nil {
			return "", fmt.Errorf("error writing json: %w", err)
		}
		return fmt.Sprintf("create environment %q", c.Environment), file.Close()
	}
}

type QueueApplicationVersion struct {
	Environment string
	Application string
	Version     uint64
}

func (c *QueueApplicationVersion) Transform(ctx context.Context, fs billy.Filesystem) (string, error) {
	// Create a symlink to the release
	applicationDir := fs.Join("environments", c.Environment, "applications", c.Application)
	if err := fs.MkdirAll(applicationDir, 0777); err != nil {
		return "", err
	}
	queuedVersionFile := fs.Join(applicationDir, queueFileName)
	if err := fs.Remove(queuedVersionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, c.Version)
	if err := fs.Symlink(fs.Join("..", "..", "..", "..", releaseDir), queuedVersionFile); err != nil {
		return "", err
	}

	// TODO SU: maybe check here if that version is already deployed? or somewhere else ... or not at all...

	return fmt.Sprintf("Queued version %d of app %q in env %q", c.Version, c.Application, c.Environment), nil
}

type DeployApplicationVersion struct {
	Environment   string
	Application   string
	Version       uint64
	LockBehaviour api.LockBehavior
}

func (c *DeployApplicationVersion) Transform(ctx context.Context, fs billy.Filesystem) (string, error) {
	// Check that the release exist and fetch manifest
	releaseDir := releasesDirectoryWithVersion(fs, c.Application, c.Version)
	manifest := fs.Join(releaseDir, "environments", c.Environment, "manifests.yaml")
	manifestContent := []byte{}
	if file, err := fs.Open(manifest); err != nil {
		return "", wrapFileError(err, manifest, "could not open manifest")
	} else {
		if content, err := io.ReadAll(file); err != nil {
			return "", err
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
		envLocks, err = (&State{Filesystem: fs}).GetEnvironmentLocks(c.Environment)
		if err != nil {
			return "", err
		}
		appLocks, err = (&State{Filesystem: fs}).GetEnvironmentApplicationLocks(c.Environment, c.Application)
		if err != nil {
			return "", err
		}
		if len(envLocks) > 0 || len(appLocks) > 0 {
			switch c.LockBehaviour {
			case api.LockBehavior_Queue:
				q := QueueApplicationVersion{
					Environment: c.Environment,
					Application: c.Application,
					Version:     c.Version,
				}
				return q.Transform(ctx, fs)
			case api.LockBehavior_Fail:
				return "", &LockedError{
					EnvironmentApplicationLocks: appLocks,
					EnvironmentLocks:            envLocks,
				}
			case api.LockBehavior_Ignore:
				// just continue
			}
		}
	}
	// Create a symlink to the release
	applicationDir := fs.Join("environments", c.Environment, "applications", c.Application)
	if err := fs.MkdirAll(applicationDir, 0777); err != nil {
		return "", err
	}
	versionFile := fs.Join(applicationDir, "version")
	if err := fs.Remove(versionFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := fs.Symlink(fs.Join("..", "..", "..", "..", releaseDir), versionFile); err != nil {
		return "", err
	}
	// Copy the manifest for argocd
	manifestsDir := fs.Join(applicationDir, "manifests")
	if err := fs.MkdirAll(manifestsDir, 0777); err != nil {
		return "", err
	}
	manifestFilename := fs.Join(manifestsDir, "manifests.yaml")
	if err := util.WriteFile(fs, manifestFilename, manifestContent, 0666); err != nil {
		return "", err
	}
	s := State{
		Filesystem: fs,
	}
	err := s.DeleteQueuedVersionIfExists(c.Environment, c.Application)
	if err != nil {
		return "", err
	}
	d := &CleanupOldApplicationVersions{
		Application: c.Application,
	}
	transform, err := d.Transform(ctx, fs)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("deployed version %d of %q to %q\n%s", c.Version, c.Application, c.Environment, transform), nil
}

type ReleaseTrain struct {
	Environment string
	Team        string
}

func (s *State) GetApplicationTeamOwner(application string) (string, error) {
	appDir := applicationDirectory(s.Filesystem, application)
	appTeam := s.Filesystem.Join(appDir, "team")

	if team, err := readFile(s.Filesystem, appTeam); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		} else {
			return "", fmt.Errorf("error while reading team owner file for application %v found: %w", application, err)
		}
	} else {
		return string(team), nil
	}
}

func (c *ReleaseTrain) Transform(ctx context.Context, fs billy.Filesystem) (string, error) {
	var state = &State{Filesystem: fs}
	var targetEnvName = c.Environment

	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return "", err
	}
	envConfig, ok := configs[targetEnvName]
	if !ok {
		return "", fmt.Errorf("could not find environment config for '%v'", targetEnvName)
	}
	if envConfig.Upstream == nil {
		return fmt.Sprintf("Environment %q does not have upstream configured - exiting.", targetEnvName), nil
	}
	var upstreamEnvName = envConfig.Upstream.Environment
	if upstreamEnvName == "" {
		return fmt.Sprintf("Environment %q does not have upstream environment configured - exiting.", targetEnvName), nil
	}
	_, ok = configs[upstreamEnvName]
	if !ok {
		return fmt.Sprintf("Could not find environment config for upstream env %q. Target env was %q", upstreamEnvName, targetEnvName), err
	}

	envLocks, err := state.GetEnvironmentLocks(targetEnvName)
	if err != nil {
		return "", fmt.Errorf("could not get lock for environment %q: %w", targetEnvName, err)
	}
	if len(envLocks) > 0 {
		return fmt.Sprintf("Target Environment '%s' is locked - exiting.", targetEnvName), nil
	}

	apps, err := state.GetEnvironmentApplications(upstreamEnvName)
	if err != nil {
		return "", fmt.Errorf("environment applications for %q not found: %w", upstreamEnvName, err)
	}

	// now iterate over all apps, deploying all that are not locked
	numServices := 0
	completeMessage := ""
	for _, appName := range apps {
		if c.Team != "" {
			if team, err := state.GetApplicationTeamOwner(appName); err != nil {
				return "", nil
			} else if c.Team != team {
				continue
			}
		}
		currentlyDeployedVersion, err := state.GetEnvironmentApplicationVersion(targetEnvName, appName)
		if err != nil {
			return "", fmt.Errorf("could not get version of application %q in env %q: %w", appName, targetEnvName, err)
		}
		versionToDeploy, err := state.GetEnvironmentApplicationVersion(upstreamEnvName, appName)
		if err != nil {
			return "", fmt.Errorf("could not get version of application %q in env %q: %w", appName, upstreamEnvName, err)
		}
		if versionToDeploy == nil {
			completeMessage = fmt.Sprintf("skipping because there is no version for application %q in env %q", appName, upstreamEnvName)
			continue
		}
		if currentlyDeployedVersion != nil && *currentlyDeployedVersion == *versionToDeploy {
			completeMessage = fmt.Sprintf("%sskipping %q because it is already in the version %d\n", completeMessage, appName, *currentlyDeployedVersion)
			continue
		}

		d := &DeployApplicationVersion{
			Environment:   targetEnvName, // here we deploy to the next env
			Application:   appName,
			Version:       *versionToDeploy,
			LockBehaviour: api.LockBehavior_Queue,
		}
		transform, err := d.Transform(ctx, fs)
		if err != nil {
			_, ok := err.(*LockedError)
			if ok {
				continue // locked errors are to be expected
			}
			if errors.Is(err, os.ErrNotExist) {
				continue // some apps do not exist on all envs, we ignore those
			}
			return "", fmt.Errorf("unexpected error while deploying app %q to env %q: %w", appName, upstreamEnvName, err)
		}
		numServices += 1
		completeMessage = completeMessage + transform + "\n"
	}
	return fmt.Sprintf("The release train deployed %d services from '%s' to '%s'\n%s\n", numServices, upstreamEnvName, targetEnvName, completeMessage), nil
}
