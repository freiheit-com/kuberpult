module github.com/freiheit-com/kuberpult

go 1.25.0

require (
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-cmp v0.7.0
	github.com/stretchr/testify v1.11.1 // indirect
	// versions for k8s dependencies should not be updated
	k8s.io/apimachinery v0.33.1
	k8s.io/utils v0.0.0-20250502105355-0f33e8f1c979 // indirect
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	golang.org/x/sync v0.19.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.32.5
)

require (
	github.com/dprotaso/go-yit v0.0.0-20220510233725-9ba8df137936 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/getkin/kin-openapi v0.131.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-test/deep v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/oapi-codegen/oapi-codegen/v2 v2.4.1 // indirect
	github.com/oasdiff/yaml v0.0.0-20250309154309-f31be36b4037 // indirect
	github.com/oasdiff/yaml3 v0.0.0-20250309153720-d2182401db90 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.36.2 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/speakeasy-api/openapi-overlay v0.9.0 // indirect
	github.com/ugorji/go/codec v1.2.14 // indirect
	github.com/vmware-labs/yaml-jsonpath v0.3.2 // indirect
	golang.org/x/mod v0.32.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	golang.org/x/tools v0.41.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.7.0 // indirect
)

replace (
	github.com/chai2010/gettext-go => github.com/chai2010/gettext-go v1.0.2 // indirect
	// https://github.com/kubernetes/kubernetes/issues/79384#issuecomment-505627280
	k8s.io/api => k8s.io/api v0.29.7
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.29.7
	k8s.io/apimachinery => k8s.io/apimachinery v0.29.11
	k8s.io/apiserver => k8s.io/apiserver v0.29.7
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.29.7
	k8s.io/client-go => k8s.io/client-go v0.29.7
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.29.7
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.29.7
	k8s.io/code-generator => k8s.io/code-generator v0.29.11
	k8s.io/component-base => k8s.io/component-base v0.29.7
	k8s.io/component-helpers => k8s.io/component-helpers v0.29.7
	k8s.io/controller-manager => k8s.io/controller-manager v0.29.7
	k8s.io/cri-api => k8s.io/cri-api v0.29.11
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.29.7
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.29.7
	k8s.io/endpointslice => k8s.io/endpointslice v0.29.7
	k8s.io/kms => k8s.io/kms v0.29.7
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.29.7
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.29.7
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.29.7
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.29.7
	k8s.io/kubectl => k8s.io/kubectl v0.29.7
	k8s.io/kubelet => k8s.io/kubelet v0.29.7
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.29.7
	k8s.io/metrics => k8s.io/metrics v0.29.7
	k8s.io/mount-utils => k8s.io/mount-utils v0.29.7
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.29.7
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.29.7
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.29.7
	k8s.io/sample-controller => k8s.io/sample-controller v0.29.7
)

tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
