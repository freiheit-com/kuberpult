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
package cmd

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/kelseyhightower/envconfig"
	"golang.org/x/crypto/openpgp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/freiheit-com/fdc-continuous-delivery/pkg/api"
	"github.com/freiheit-com/fdc-continuous-delivery/pkg/setup"
	"github.com/freiheit-com/fdc-continuous-delivery/services/cd-service/pkg/repository"
	"github.com/freiheit-com/fdc-continuous-delivery/services/cd-service/pkg/service"
)

type Config struct {
	// these will be mapped to "KUBERPULT_GIT_URL", etc.
	GitUrl            string `required:"true" split_words:"true"`
	GitBranch         string `default:"master" split_words:"true"`
	GitCommitterEmail string `default:"kuberpult@freiheit.com" split_words:"true"`
	GitCommitterName  string `default:"kuberpult" split_words:"true"`
	GitSshKey         string `default:"/etc/ssh/identity" split_words:"true"`
	GitSshKnownHosts  string `default:"/etc/ssh/ssh_known_hosts" split_words:"true"`
	PgpKeyRing        string `split_words:"true"`
	ArgoCdHost        string `default:"localhost:8080" split_words:"true"`
	ArgoCdUser        string `default:"admin" split_words:"true"`
	ArgoCdPass        string `default:"" split_words:"true"`
}

func (c *Config) readPgpKeyRing() (openpgp.KeyRing, error) {
	if c.PgpKeyRing == "" {
		return nil, nil
	}
	file, err := os.Open(c.PgpKeyRing)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return openpgp.ReadArmoredKeyRing(file)

}

func RunServer() {
	var c Config
	err := envconfig.Process("kuberpult", &c)
	if err != nil {
		log.Fatal(err.Error())
	}

	pgpKeyRing, err := c.readPgpKeyRing()
	if err != nil {
		log.Printf("error reading pgp key ring: %s\n", err)
		return
	}

	if c.ArgoCdPass != "" {
		_, err := service.ArgocdLogin(c.ArgoCdHost, c.ArgoCdUser, c.ArgoCdPass)
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	ctx := context.Background()
	repo, err := repository.New(ctx, repository.Config{
		URL:            c.GitUrl,
		Path:           "./repository",
		CommitterEmail: c.GitCommitterEmail,
		CommitterName:  c.GitCommitterName,
		Credentials: repository.Credentials{
			SshKey: c.GitSshKey,
		},
		Certificates: repository.Certificates{
			KnownHostsFile: c.GitSshKnownHosts,
		},
		Branch: c.GitBranch,
	})
	if err != nil {
		log.Fatal(err.Error())
	}

	lockSrv := &service.LockServiceServer{
		Repository: repo,
	}
	deploySrv := &service.DeployServiceServer{
		Repository: repo,
	}

	grpcProxy := runtime.NewServeMux()
	err = api.RegisterLockServiceHandlerServer(ctx, grpcProxy, lockSrv)
	if err != nil {
		log.Fatal(err.Error())
	}
	err = api.RegisterDeployServiceHandlerServer(ctx, grpcProxy, deploySrv)
	if err != nil {
		log.Fatal(err.Error())
	}

	repositoryService := &service.Service{
		Repository: repo,
		KeyRing:    pgpKeyRing,
		ArgoCdHost: c.ArgoCdHost,
		ArgoCdUser: c.ArgoCdUser,
		ArgoCdPass: c.ArgoCdPass,
	}

	// Shutdown channel is used to terminate server side streams.
	shutdownCh := make(chan struct{})
	setup.Run(setup.Config{
		HTTP: []setup.HTTPConfig{
			{
				Port: "8080",
				Register: func(mux *http.ServeMux) {
					mux.Handle("/release", repositoryService)
					mux.Handle("/health", repositoryService)
					mux.Handle("/sync/", repositoryService)
					mux.Handle("/", grpcProxy)
				},
			},
		},
		GRPC: &setup.GRPCConfig{
			Port: "8443",
			Register: func(srv *grpc.Server) {
				api.RegisterLockServiceServer(srv, lockSrv)

				api.RegisterDeployServiceServer(srv, deploySrv)

				envSrv := &service.EnvironmentServiceServer{
					Repository: repo,
				}
				api.RegisterEnvironmentServiceServer(srv, envSrv)

				overviewSrv := &service.OverviewServiceServer{
					Repository: repo,
					Shutdown:   shutdownCh,
				}
				api.RegisterOverviewServiceServer(srv, overviewSrv)
				reflection.Register(srv)
			},
		},
		Shutdown: func(ctx context.Context) error {
			close(shutdownCh)
			return nil
		},
	})
}
