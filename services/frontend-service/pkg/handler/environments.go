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
