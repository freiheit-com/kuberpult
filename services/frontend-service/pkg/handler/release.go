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
	"net/http/httputil"
	"net/url"
)

func (s Server) HandleRelease(w http.ResponseWriter, req *http.Request, tail string) {
	if tail != "/" {
		http.Error(w, fmt.Sprintf("Release does not accept additional path arguments, got: %s", tail), http.StatusNotFound)
		return
	}
	url, err := url.Parse(s.Config.HttpCdServer + "/release")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Release api requires file uploads in it, which is hard to implement in grpc.
	// Release api in cd-service only exists in REST right now, so this endpoint directly calls the REST endpoint of cd-service instead of a grpc one.
	cdServiceProxy := httputil.NewSingleHostReverseProxy(url)
	cdServiceProxy.ServeHTTP(w, req)
}
