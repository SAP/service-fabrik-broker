module github.com/cloudfoundry-incubator/service-fabrik-broker/operator-apis

go 1.15

require (
	github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator v0.0.0-00010101000000-000000000000
	github.com/go-logr/logr v0.2.0
	github.com/gorilla/mux v1.8.0
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v0.18.8
	sigs.k8s.io/controller-runtime v0.6.3
)

replace github.com/cloudfoundry-incubator/service-fabrik-broker/interoperator => ../interoperator
