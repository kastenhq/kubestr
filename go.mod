module github.com/kastenhq/kubestr

go 1.14

replace (
	github.com/graymeta/stow => github.com/kastenhq/stow v0.1.2-kasten
	github.com/kanisterio/kanister => github.com/kanisterio/kanister v0.0.0-20201019101000-6d342798b895
)

require (
	github.com/golang/mock v1.4.4
	github.com/kanisterio/kanister v0.0.0-00010101000000-000000000000
	github.com/kubernetes-csi/external-snapshotter v1.2.2
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.0.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	k8s.io/api v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v0.19.0
)
