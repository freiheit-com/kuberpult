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

package reposerver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argorepo "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository"
	"github.com/go-git/go-billy/v5/util"
	git "github.com/libgit2/git2go/v34"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type reposerver struct {
	repo   repository.Repository
	config repository.RepositoryConfig
}

var resourceTracking argo.ResourceTracking = argo.NewResourceTracking()
var notImplemented error = status.Error(codes.Unimplemented, "not implemented")

// GenerateManifest implements apiclient.RepoServerServiceServer.
func (r *reposerver) GenerateManifest(ctx context.Context, req *argorepo.ManifestRequest) (*argorepo.ManifestResponse, error) {
	state, _, err := r.resolve(req.Revision)
	if err != nil {
		return nil, err
	}
	bfs := state.Filesystem
	path := req.ApplicationSource.Path
	mn := []string{}
	filter := func(p string) bool { return true }
	if req.ApplicationSource.Directory != nil {
		if req.ApplicationSource.Directory.Include != "" {
			inc := req.ApplicationSource.Directory.Include
			filter = func(p string) bool { return p == inc }
		}
	}
	err = util.Walk(bfs, path, func(file string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filter(info.Name()) {
			m, err := util.ReadFile(bfs, file)
			if err != nil {
				return fmt.Errorf("reading %s: %w", file, err)
			}
			parts, err := kube.SplitYAML(m)
			if err != nil {
				return err
			}
			for _, obj := range parts {
				if req.AppLabelKey != "" && req.AppName != "" && !kube.IsCRD(obj) {
					err = resourceTracking.SetAppInstance(obj, req.AppLabelKey, req.AppName, req.Namespace, v1alpha1.TrackingMethod(req.TrackingMethod))
					if err != nil {
						return err
					}
				}
				jsonData, err := json.Marshal(obj.Object)
				if err != nil {
					return err
				}
				mn = append(mn, string(jsonData))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	resp := &argorepo.ManifestResponse{
		Namespace:            "",
		Server:               "",
		VerifyResult:         "",
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
		Manifests:            mn,
		Revision:             state.Commit.String(),
		SourceType:           "Directory",
	}
	return resp, nil
}

// GenerateManifestWithFiles implements apiclient.RepoServerServiceServer.
func (*reposerver) GenerateManifestWithFiles(argorepo.RepoServerService_GenerateManifestWithFilesServer) error {
	return notImplemented
}

// GetAppDetails implements apiclient.RepoServerServiceServer.
func (*reposerver) GetAppDetails(context.Context, *argorepo.RepoServerAppDetailsQuery) (*argorepo.RepoAppDetailsResponse, error) {
	return nil, notImplemented
}

// GetHelmCharts implements apiclient.RepoServerServiceServer.
func (*reposerver) GetHelmCharts(context.Context, *argorepo.HelmChartsRequest) (*argorepo.HelmChartsResponse, error) {
	return nil, notImplemented
}

// GetRevisionMetadata implements apiclient.RepoServerServiceServer.
func (*reposerver) GetRevisionMetadata(ctx context.Context, req *argorepo.RepoServerRevisionMetadataRequest) (*v1alpha1.RevisionMetadata, error) {
	// It doesn't matter too much what is in here as long as we don't give an error.
	return &v1alpha1.RevisionMetadata{
		Author: "",
		Date: v1.Time{
			Time: time.Time{},
		},
		Tags:          nil,
		Message:       "",
		SignatureInfo: "",
	}, nil
}

// ListApps implements apiclient.RepoServerServiceServer.
func (*reposerver) ListApps(context.Context, *argorepo.ListAppsRequest) (*argorepo.AppList, error) {
	return nil, notImplemented
}

// ListPlugins implements apiclient.RepoServerServiceServer.
func (*reposerver) ListPlugins(context.Context, *emptypb.Empty) (*argorepo.PluginList, error) {
	return nil, notImplemented
}

// ListRefs implements apiclient.RepoServerServiceServer.
func (*reposerver) ListRefs(context.Context, *argorepo.ListRefsRequest) (*argorepo.Refs, error) {
	return nil, notImplemented
}

// ResolveRevision implements apiclient.RepoServerServiceServer.
func (r *reposerver) ResolveRevision(ctx context.Context, req *argorepo.ResolveRevisionRequest) (*argorepo.ResolveRevisionResponse, error) {
	state, oid, err := r.resolve(req.AmbiguousRevision)
	if err != nil {
		// This looks a bit strange but argocd actually responds with a succes response if the ambiguous ref can be parsed as a git commit id even if that commit does not exists at all.
		if serr, ok := status.FromError(err); ok && serr.Code() == codes.NotFound && oid != nil {
			return &argorepo.ResolveRevisionResponse{
				XXX_NoUnkeyedLiteral: struct{}{},
				XXX_unrecognized:     nil,
				XXX_sizecache:        0,
				Revision:             oid.String(),
				AmbiguousRevision:    fmt.Sprintf("%s (%s)", req.AmbiguousRevision, oid),
			}, nil
		}
		return nil, err
	}
	resp := argorepo.ResolveRevisionResponse{
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
		Revision:             state.Commit.String(),
		AmbiguousRevision:    fmt.Sprintf("%s (%s)", req.AmbiguousRevision, state.Commit),
	}
	return &resp, nil
}

func (r *reposerver) resolve(rev string) (*repository.State, *git.Oid, error) {
	var oid *git.Oid
	if o, err := git.NewOid(rev); err == nil {
		oid = o
	} else if rev != "HEAD" && rev != r.config.Branch {
		return nil, nil, status.Error(codes.NotFound, fmt.Sprintf("unknown revision %q, I only know \"HEAD\", %q and commit hashes", rev, r.config.Branch))
	}
	state, err := r.repo.StateAt(oid)
	if err != nil {
		var gerr *git.GitError
		if errors.As(err, &gerr) {
			if gerr.Code == git.ErrorCodeNotFound {

				return nil, oid, status.Error(codes.NotFound, fmt.Sprintf("unknown revision %q, I only know \"HEAD\", %q and commit hashes", rev, r.config.Branch))
			}
		}
		return nil, oid, status.Error(codes.Internal, fmt.Sprintf("internal error: %s", err))
	}
	return state, oid, nil
}

// TestRepository implements apiclient.RepoServerServiceServer.
func (*reposerver) TestRepository(context.Context, *argorepo.TestRepositoryRequest) (*argorepo.TestRepositoryResponse, error) {
	return nil, notImplemented
}

func (*reposerver) GetGitDirectories(context.Context, *argorepo.GitDirectoriesRequest) (*argorepo.GitDirectoriesResponse, error) {
	return nil, notImplemented
}

func (*reposerver) GetGitFiles(context.Context, *argorepo.GitFilesRequest) (*argorepo.GitFilesResponse, error) {
	return nil, notImplemented
}

func (*reposerver) GetRevisionChartDetails(context.Context, *argorepo.RepoServerRevisionChartDetailsRequest) (*v1alpha1.ChartDetails, error) {
	return nil, notImplemented
}

func New(repo repository.Repository, config repository.RepositoryConfig) argorepo.RepoServerServiceServer {
	return &reposerver{repo, config}
}

func Register(s *grpc.Server, repo repository.Repository, config repository.RepositoryConfig) {
	argorepo.RegisterRepoServerServiceServer(s, New(repo, config))
}
