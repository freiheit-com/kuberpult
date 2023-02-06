#This file is part of kuberpult.

#Kuberpult is free software: you can redistribute it and/or modify
#it under the terms of the Expat(MIT) License as published by
#the Free Software Foundation.

#Kuberpult is distributed in the hope that it will be useful,
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#MIT License for more details.

#You should have received a copy of the MIT License
#along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

#Copyright 2023 freiheit.com


api_grpc.pb.go api.pb.go api.pb.gw.go: api.proto buf.yaml buf.gen.yaml buf.lock
	if grep -q '^import "google/api/annotations.proto"' $< || grep -q '^import "google/api/http.proto"' $< ; then echo 'Instead of using google.api.http protobuf annotations we write HTTP handlers by hand' ; exit 1 ; fi
	buf generate

clean:
	rm -f api_grpc.pb.go api.pb.go api.pb.gw.go

release: #no-op
test: #no op

all: api_grpc.pb.go

.PHONY: clean all release
