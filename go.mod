module github.com/Jason-ZW/autok3s

go 1.13

replace (
	k8s.io/api => github.com/rancher/kubernetes/staging/src/k8s.io/api v1.18.6-k3s1
	k8s.io/apiextensions-apiserver => github.com/rancher/kubernetes/staging/src/k8s.io/apiextensions-apiserver v1.18.6-k3s1
	k8s.io/apimachinery => github.com/rancher/kubernetes/staging/src/k8s.io/apimachinery v1.18.6-k3s1
	k8s.io/apiserver => github.com/rancher/kubernetes/staging/src/k8s.io/apiserver v1.18.6-k3s1
	k8s.io/cli-runtime => github.com/rancher/kubernetes/staging/src/k8s.io/cli-runtime v1.18.6-k3s1
	k8s.io/client-go => github.com/rancher/kubernetes/staging/src/k8s.io/client-go v1.18.6-k3s1
	k8s.io/cloud-provider => github.com/rancher/kubernetes/staging/src/k8s.io/cloud-provider v1.18.6-k3s1
	k8s.io/cluster-bootstrap => github.com/rancher/kubernetes/staging/src/k8s.io/cluster-bootstrap v1.18.6-k3s1
	k8s.io/code-generator => github.com/rancher/kubernetes/staging/src/k8s.io/code-generator v1.18.6-k3s1
	k8s.io/component-base => github.com/rancher/kubernetes/staging/src/k8s.io/component-base v1.18.6-k3s1
	k8s.io/cri-api => github.com/rancher/kubernetes/staging/src/k8s.io/cri-api v1.18.6-k3s1
	k8s.io/csi-translation-lib => github.com/rancher/kubernetes/staging/src/k8s.io/csi-translation-lib v1.18.6-k3s1
	k8s.io/kube-aggregator => github.com/rancher/kubernetes/staging/src/k8s.io/kube-aggregator v1.18.6-k3s1
	k8s.io/kube-controller-manager => github.com/rancher/kubernetes/staging/src/k8s.io/kube-controller-manager v1.18.6-k3s1
	k8s.io/kube-proxy => github.com/rancher/kubernetes/staging/src/k8s.io/kube-proxy v1.18.6-k3s1
	k8s.io/kube-scheduler => github.com/rancher/kubernetes/staging/src/k8s.io/kube-scheduler v1.18.6-k3s1
	k8s.io/kubectl => github.com/rancher/kubernetes/staging/src/k8s.io/kubectl v1.18.6-k3s1
	k8s.io/kubelet => github.com/rancher/kubernetes/staging/src/k8s.io/kubelet v1.18.6-k3s1
	k8s.io/kubernetes => github.com/rancher/kubernetes v1.18.6-k3s1
	k8s.io/legacy-cloud-providers => github.com/rancher/kubernetes/staging/src/k8s.io/legacy-cloud-providers v1.18.6-k3s1
	k8s.io/metrics => github.com/rancher/kubernetes/staging/src/k8s.io/metrics v1.18.6-k3s1
	k8s.io/node-api => github.com/rancher/kubernetes/staging/src/k8s.io/node-api v1.18.6-k3s1
	k8s.io/sample-apiserver => github.com/rancher/kubernetes/staging/src/k8s.io/sample-apiserver v1.18.6-k3s1
	k8s.io/sample-cli-plugin => github.com/rancher/kubernetes/staging/src/k8s.io/sample-cli-plugin v1.18.6-k3s1
	k8s.io/sample-controller => github.com/rancher/kubernetes/staging/src/k8s.io/sample-controller v1.18.6-k3s1
	mvdan.cc/unparam => mvdan.cc/unparam v0.0.0-20190209190245-fbb59629db34
)

require (
	github.com/alexellis/go-execute v0.0.0-20200124154445-8697e4e28c5e
	github.com/aliyun/alibaba-cloud-sdk-go v1.61.381
	github.com/briandowns/spinner v1.11.1
	github.com/docker/docker v0.7.3-0.20190327010347-be7ac8be2ae0
	github.com/ghodss/yaml v1.0.0
	github.com/morikuni/aec v1.0.0
	github.com/olekukonko/tablewriter v0.0.4
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.4.0
	golang.org/x/crypto v0.0.0-20200220183623-bac4c82f6975
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/component-base v0.0.0
	k8s.io/kubernetes v0.0.0-00010101000000-000000000000
)
