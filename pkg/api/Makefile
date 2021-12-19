#This file is part of kuberpult.

#Kuberpult is free software: you can redistribute it and/or modify
#it under the terms of the GNU General Public License as published by
#the Free Software Foundation, either version 3 of the License, or
#(at your option) any later version.

#Kuberpult is distributed in the hope that it will be useful,
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU General Public License for more details.

#You should have received a copy of the GNU General Public License
#along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

#Copyright 2021 freiheit.com

api_grpc.pb.go api.pb.go api.pb.gw.go: api.proto buf.yaml buf.gen.yaml
	buf generate

clean:
	rm -f api_grpc.pb.go api.pb.go api.pb.gw.go

release: #no-op
test: #no op

all: api_grpc.pb.go

.PHONY: clean all release
