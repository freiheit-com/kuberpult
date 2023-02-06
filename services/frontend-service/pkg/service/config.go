

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
		SourceRepoUrl:    c.Config.SourceRepoUrl,
		KuberpultVersion: c.Config.KuberpultVersion,
	}
	return &result, nil
}
