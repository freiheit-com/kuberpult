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

package cmd

import (
	"encoding/json"
	"fmt"
	sorting2 "github.com/freiheit-com/kuberpult/cli/pkg/sorting"
	"github.com/google/go-cmp/cmp"
	"log"
	"os"

	"github.com/freiheit-com/kuberpult/cli/pkg/cli_utils"
	"github.com/freiheit-com/kuberpult/cli/pkg/deployments"
	"github.com/freiheit-com/kuberpult/cli/pkg/environments"
	"github.com/freiheit-com/kuberpult/cli/pkg/locks"
	"github.com/freiheit-com/kuberpult/cli/pkg/releasetrain"

	kutil "github.com/freiheit-com/kuberpult/cli/pkg/kuberpult_utils"
	rl "github.com/freiheit-com/kuberpult/cli/pkg/release"
)

func handleRelease(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := rl.ParseArgs(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}

	if parsedArgs.DryRun {
		_, err := fmt.Fprintf(os.Stderr, "This is a dry-run.\n"+
			"I will print out the diff of the manifests you supplied, compared to the latest release in kuberpults database.\n"+
			"The minus sign (-) stands for the existing release in kuberpults DB, and the plus sign (+) stands for the manifests you supplied as parameters.\n")
		if err != nil {
			panic(err)
		}
	}

	authParams := kutil.AuthenticationParameters{
		IapToken:    kpClientParams.iapToken,
		DexToken:    kpClientParams.dexToken,
		AuthorName:  kpClientParams.authorName,
		AuthorEmail: kpClientParams.authorEmail,
	}

	requestParameters := kutil.RequestParameters{
		Url:         &kpClientParams.url,
		Retries:     kpClientParams.retries,
		HttpTimeout: cli_utils.HttpDefaultTimeout,
	}

	if parsedArgs.DryRun {
		return HandleReleaseDiff(requestParameters, authParams, *parsedArgs)
	} else {
		if err = rl.Release(requestParameters, authParams, *parsedArgs); err != nil {
			log.Printf("error on release, error: %v", err)
			return ReturnCodeFailure
		}
	}
	return ReturnCodeSuccess
}

func HandleReleaseDiff(kpClientParams kutil.RequestParameters, args kutil.AuthenticationParameters, parsedArgs rl.ReleaseParameters) ReturnCode {
	body, err := rl.GetManifests(kpClientParams, args, parsedArgs)
	if err != nil {
		log.Printf("error on getting manifests, error: %v", err)
		return ReturnCodeFailure
	}

	var resp = &GetManifestsResponse{
		Manifests: nil,
	}
	err = json.Unmarshal([]byte(body), &resp)
	if err != nil {
		log.Printf("error while unmarshalling manifests, error: %v", err)
		return ReturnCodeFailure
	}

	// We sort by environment name to maintain a stable sorting:
	sortedKeys := sorting2.SortKeys(parsedArgs.Manifests)
	for keyIndex := range sortedKeys {
		envName := sortedKeys[keyIndex]
		inputManifestBytes := parsedArgs.Manifests[envName]
		inputManifest := string(inputManifestBytes)
		val, ok := resp.Manifests[envName]
		var gottenManifest string
		if !ok {
			gottenManifest = ""
		} else {
			gottenManifest = val.Content
		}
		diff := cmp.Diff(gottenManifest, inputManifest)
		fmt.Printf("Diff for the manifest on environment '%s':\n%s", envName, diff)
	}

	return ReturnCodeSuccess
}

func handleReleaseTrain(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := releasetrain.ParseArgsReleaseTrain(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}

	authParams := kutil.AuthenticationParameters{
		IapToken:    kpClientParams.iapToken,
		DexToken:    kpClientParams.dexToken,
		AuthorName:  kpClientParams.authorName,
		AuthorEmail: kpClientParams.authorEmail,
	}

	requestParameters := kutil.RequestParameters{
		Url:         &kpClientParams.url,
		Retries:     kpClientParams.retries,
		HttpTimeout: int(kpClientParams.timeout),
	}

	if err = releasetrain.HandleReleaseTrain(requestParameters, authParams, *parsedArgs); err != nil {
		log.Printf("error on release train, error: %v", err)
		return ReturnCodeFailure
	}
	return ReturnCodeSuccess
}

func handleDeleteEnvLock(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := locks.ParseArgsDeleteEnvironmentLock(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}
	return handleLockRequest(kpClientParams, parsedArgs)
}

func handleCreateEnvLock(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := locks.ParseArgsCreateEnvironmentLock(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}
	return handleLockRequest(kpClientParams, parsedArgs)
}

func handleDeleteAppLock(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := locks.ParseArgsDeleteAppLock(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}
	return handleLockRequest(kpClientParams, parsedArgs)
}

func handleCreateAppLock(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := locks.ParseArgsCreateAppLock(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}
	return handleLockRequest(kpClientParams, parsedArgs)
}

func handleDeleteTeamLock(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := locks.ParseArgsDeleteTeamLock(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}
	return handleLockRequest(kpClientParams, parsedArgs)
}

func handleCreateTeamLock(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := locks.ParseArgsCreateTeamLock(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}
	return handleLockRequest(kpClientParams, parsedArgs)
}

func handleCreateGroupLock(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := locks.ParseArgsCreateGroupLock(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}
	return handleLockRequest(kpClientParams, parsedArgs)
}

func handleDeleteGroupLock(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := locks.ParseArgsDeleteGroupLock(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}
	return handleLockRequest(kpClientParams, parsedArgs)
}

func handleLockRequest(kpClientParams kuberpultClientParameters, parsedArgs locks.LockParameters) ReturnCode {
	authParams := kutil.AuthenticationParameters{
		IapToken:    kpClientParams.iapToken,
		DexToken:    kpClientParams.dexToken,
		AuthorName:  kpClientParams.authorName,
		AuthorEmail: kpClientParams.authorEmail,
	}

	requestParameters := kutil.RequestParameters{
		Url:         &kpClientParams.url,
		Retries:     kpClientParams.retries,
		HttpTimeout: cli_utils.HttpDefaultTimeout,
	}

	if err := locks.HandleLockRequest(requestParameters, authParams, parsedArgs); err != nil {
		log.Printf("error creating lock, error: %v", err)
		return ReturnCodeFailure
	}
	return ReturnCodeSuccess
}

func handleGetCommitDeployments(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := deployments.ParseArgsCommitDeployments(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}

	authParams := kutil.AuthenticationParameters{
		IapToken:    kpClientParams.iapToken,
		DexToken:    kpClientParams.dexToken,
		AuthorName:  kpClientParams.authorName,
		AuthorEmail: kpClientParams.authorEmail,
	}

	requestParameters := kutil.RequestParameters{
		Url:         &kpClientParams.url,
		Retries:     kpClientParams.retries,
		HttpTimeout: cli_utils.HttpDefaultTimeout,
	}

	if err = deployments.HandleGetCommitDeployments(requestParameters, authParams, parsedArgs); err != nil {
		log.Printf("error on commit deployments, error: %v", err)
		return ReturnCodeFailure
	}
	return ReturnCodeSuccess
}

func handleGetDeploymentCommit(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := deployments.ParseArgsDeploymentCommit(args)

	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}

	authParams := kutil.AuthenticationParameters{
		IapToken:    kpClientParams.iapToken,
		DexToken:    kpClientParams.dexToken,
		AuthorName:  kpClientParams.authorName,
		AuthorEmail: kpClientParams.authorEmail,
	}

	requestParameters := kutil.RequestParameters{
		Url:         &kpClientParams.url,
		Retries:     kpClientParams.retries,
		HttpTimeout: cli_utils.HttpDefaultTimeout,
	}

	if err = deployments.HandleGetDeploymentCommit(requestParameters, authParams, parsedArgs); err != nil {
		log.Printf("error on commit deployments, error: %v", err)
		return ReturnCodeFailure
	}
	return ReturnCodeSuccess
}

func handleDeleteEnvironment(kpClientParams kuberpultClientParameters, args []string) ReturnCode {
	parsedArgs, err := environments.ParseArgsDeleteEnvironment(args)
	if err != nil {
		log.Printf("error while parsing command line args, error: %v", err)
		return ReturnCodeInvalidArguments
	}

	authParams := kutil.AuthenticationParameters{
		IapToken:    kpClientParams.iapToken,
		DexToken:    kpClientParams.dexToken,
		AuthorName:  kpClientParams.authorName,
		AuthorEmail: kpClientParams.authorEmail,
	}

	requestParameters := kutil.RequestParameters{
		Url:         &kpClientParams.url,
		Retries:     kpClientParams.retries,
		HttpTimeout: cli_utils.HttpDefaultTimeout,
	}

	if err = environments.HandleDeleteEnvironment(requestParameters, authParams, parsedArgs); err != nil {
		log.Printf("error on delete environment, error: %v", err)
		return ReturnCodeFailure
	}

	return ReturnCodeSuccess
}
