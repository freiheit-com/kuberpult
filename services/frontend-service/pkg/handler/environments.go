
package handler

import (
	"fmt"
	"net/http"

	xpath "github.com/freiheit-com/kuberpult/pkg/path"
)

func (s Server) HandleEnvironments(w http.ResponseWriter, req *http.Request, tail string) {
	environment, tail := xpath.Shift(tail)
	if environment == "" {
		http.Error(w, "missing environment ID", http.StatusNotFound)
		return
	}

	function, tail := xpath.Shift(tail)
	switch function {
	case "applications":
		s.handleApplications(w, req, environment, tail)
	case "locks":
		s.handleEnvironmentLocks(w, req, environment, tail)
	case "releasetrain":
		s.handleReleaseTrain(w, req, environment, tail)
	default:
		http.Error(w, fmt.Sprintf("unknown function '%s'", function), http.StatusNotFound)
	}
}
