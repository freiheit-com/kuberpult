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
		s.HandleReleaseTrain(w, req, environment, tail)
	case "":
		if tail == "/" && req.Method == http.MethodPost {
			s.handleCreateEnvironment(w, req, environment, tail)
		} else {
			http.Error(w, fmt.Sprintf("unknown function '%s'", function), http.StatusNotFound)
		}
	default:
		http.Error(w, fmt.Sprintf("unknown function '%s'", function), http.StatusNotFound)
	}
}
