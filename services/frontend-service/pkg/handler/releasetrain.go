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
	"golang.org/x/crypto/openpgp"
	pgperrors "golang.org/x/crypto/openpgp/errors"
	"net/http"
	"strings"

	"github.com/freiheit-com/kuberpult/pkg/api"
)

func (s Server) handleReleaseTrain(w http.ResponseWriter, req *http.Request, environment, tail string) {
	if req.Method != http.MethodPut {
		http.Error(w, fmt.Sprintf("releasetrain only accepts method PUT, got: '%s'", req.Method), http.StatusMethodNotAllowed)
		return
	}
	if tail != "/" {
		http.Error(w, fmt.Sprintf("releasetrain does not accept additional path arguments, got: '%s'", tail), http.StatusNotFound)
		return
	}

	validSignature := false
	signature := req.Header.Get("signature")
	if signature == "" {
		w.WriteHeader(500)
		fmt.Fprintf(w, "Internal: Invalid signature: Signature is empty")
		return
	} else {
		if _, err := openpgp.CheckArmoredDetachedSignature(s.KeyRing, strings.NewReader(environment), strings.NewReader(signature)); err != nil {
			if err != pgperrors.ErrUnknownIssuer {
				w.WriteHeader(500)
				fmt.Fprintf(w, "Internal: Invalid Signature: %s", err)
				return
			}
		} else {
			validSignature = true
		}
	}
	if !validSignature {
		w.WriteHeader(400)
		fmt.Fprintf(w, "Invalid signature")
		return
	}

	_, err := s.DeployClient.ReleaseTrain(req.Context(), &api.ReleaseTrainRequest{
		Environment: environment,
	})
	if err != nil {
		handleGRPCError(req.Context(), w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
