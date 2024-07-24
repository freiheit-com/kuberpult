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
	"fmt"
	"net/http"

	xpath "github.com/freiheit-com/kuberpult/pkg/path"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
)

type Service struct {
	Repository repository.Repository
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	head, _ := xpath.Shift(r.URL.Path)
	switch head {
	case "health":
		s.ServeHTTPHealth(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Endpoint not found")
	}
}

func (s *Service) ServeHTTPHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "ok\n")
}

var _ http.Handler = (*Service)(nil)
