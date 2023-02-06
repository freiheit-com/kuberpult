
package handler

import (
	"fmt"
	"golang.org/x/crypto/openpgp"
	"net/http"

	"github.com/freiheit-com/kuberpult/pkg/api"
	xpath "github.com/freiheit-com/kuberpult/pkg/path"
	"github.com/freiheit-com/kuberpult/services/frontend-service/pkg/config"
)

type Server struct {
	DeployClient api.DeployServiceClient
	LockClient   api.LockServiceClient
	Config       config.ServerConfig
	KeyRing      openpgp.KeyRing
	AzureAuth    bool
}

func (s Server) Handle(w http.ResponseWriter, req *http.Request) {
	group, tail := xpath.Shift(req.URL.Path)
	switch group {
	case "environments":
		s.HandleEnvironments(w, req, tail)
	case "release":
		s.HandleRelease(w, req, tail)
	default:
		http.Error(w, fmt.Sprintf("unknown group '%s'", group), http.StatusNotFound)
	}
}
