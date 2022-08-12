/*
This file is part of kuberpult.

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

Copyright 2021 freiheit.com
*/
package service

import (
	"context"

	"github.com/freiheit-com/kuberpult/pkg/api"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/config"
)

type FrontendConfigServiceServer struct {
	Config config.FrontendConfig
}

func (c *FrontendConfigServiceServer) GetConfig(
	ctx context.Context,
	in *api.GetFrontendConfigRequest) (*api.GetFrontendConfigResponse, error) {
	result := api.GetFrontendConfigResponse{
		ArgoCd: &api.GetFrontendConfigResponse_ArgoCD{BaseUrl: c.Config.ArgoCd.BaseUrl},
		AuthConfig: &api.GetFrontendConfigResponse_Auth{
			AzureAuth: &api.GetFrontendConfigResponse_Auth_AzureAuthConfig{
				Enabled:       c.Config.Auth.AzureAuth.Enabled,
				ClientId:      c.Config.Auth.AzureAuth.ClientId,
				TenantId:      c.Config.Auth.AzureAuth.TenantId,
				CloudInstance: c.Config.Auth.AzureAuth.CloudInstance,
				RedirectURL:   c.Config.Auth.AzureAuth.RedirectURL,
			},
		},
		KuberpultVersion: c.Config.Version,
	}
	return &result, nil
}
