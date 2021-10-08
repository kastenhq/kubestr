module github.com/kastenhq/kubestr

go 1.14

replace github.com/graymeta/stow => github.com/kastenhq/stow v0.1.2-kasten

require (
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/briandowns/spinner v1.12.0
	github.com/golang/mock v1.6.0
	github.com/jarcoal/httpmock v1.0.5 // indirect
	github.com/kanisterio/kanister v0.0.0-20210805190523-86f566052e0e
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.0.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.1
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
)
