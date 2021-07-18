module github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator

go 1.15

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Masterminds/sprig/v3 v3.1.0
	github.com/go-logr/logr v0.4.0
	github.com/golang/mock v1.4.4
	github.com/google/go-cmp v0.4.1 // indirect
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	github.com/prometheus/client_golang v1.8.0
	helm.sh/helm/v3 v3.3.4
	k8s.io/api v0.18.8
	k8s.io/apiextensions-apiserver v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v0.18.8
	k8s.io/code-generator v0.18.8
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29 // indirect
	k8s.io/utils v0.0.0-20200619165400-6e3d28b6ed19 // indirect
	sigs.k8s.io/controller-runtime v0.6.3
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
)
