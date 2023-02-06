

api_grpc.pb.go api.pb.go api.pb.gw.go: api.proto buf.yaml buf.gen.yaml buf.lock
	if grep -q '^import "google/api/annotations.proto"' $< || grep -q '^import "google/api/http.proto"' $< ; then echo 'Instead of using google.api.http protobuf annotations we write HTTP handlers by hand' ; exit 1 ; fi
	buf generate

clean:
	rm -f api_grpc.pb.go api.pb.go api.pb.gw.go

release: #no-op
test: #no op

all: api_grpc.pb.go

.PHONY: clean all release
