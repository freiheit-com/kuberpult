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

package service

import (
	"context"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/config"
)

type FrontendConfigServiceServer struct {
	Config config.FrontendConfig
}

func (c *FrontendConfigServiceServer) GetConfig(
	ctx context.Context,
	in *api.GetFrontendConfigRequest) (*api.GetFrontendConfigResponse, error) {
	result := api.GetFrontendConfigResponse{
		ArgoCd: &api.GetFrontendConfigResponse_ArgoCD{
			BaseUrl:   c.Config.ArgoCd.BaseUrl,
			Namespace: c.Config.ArgoCd.Namespace,
		},
		AuthConfig: &api.GetFrontendConfigResponse_Auth{
			AzureAuth: &api.GetFrontendConfigResponse_Auth_AzureAuthConfig{
				Enabled:       c.Config.Auth.AzureAuth.Enabled,
				ClientId:      c.Config.Auth.AzureAuth.ClientId,
				TenantId:      c.Config.Auth.AzureAuth.TenantId,
				CloudInstance: c.Config.Auth.AzureAuth.CloudInstance,
				RedirectUrl:   c.Config.Auth.AzureAuth.RedirectURL,
			},
		},
		SourceRepoUrl:    c.Config.SourceRepoUrl,
		ManifestRepoUrl:  c.Config.ManifestRepoUrl,
		KuberpultVersion: c.Config.KuberpultVersion,
		Branch:           c.Config.Branch,
	}
	return &result, nil
}
