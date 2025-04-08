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

package cmd

/**
The types here are necessary because we do not have the protobuf types integrated in the "cli" directory.
These types are a small subset of api.proto.
*/

type Manifest struct {
	Content string `protobuf:"bytes,2,opt,name=content,proto3" json:"content,omitempty"`
}

type GetManifestsResponse struct {
	Manifests map[string]*Manifest `protobuf:"bytes,2,rep,name=manifests,proto3" json:"manifests,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}
