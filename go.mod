module github.com/cnrancher/autok3s

go 1.16

replace (
	github.com/docker/distribution => github.com/docker/distribution v2.8.0+incompatible
	github.com/docker/docker => github.com/docker/docker v20.10.12+incompatible

	github.com/harvester/harvester => github.com/harvester/harvester v0.0.2-0.20220129083315-1a861fc1c348
	github.com/knative/pkg => github.com/rancher/pkg v0.0.0-20190514055449-b30ab9de040e
	github.com/kubernetes-sigs/cri-tools => github.com/k3s-io/cri-tools v1.22.0-k3s1
	github.com/openshift/api => github.com/openshift/api v0.0.0-20191219222812-2987a591a72c
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20200521150516-05eb9880269c
	github.com/operator-framework/operator-lifecycle-manager => github.com/operator-framework/operator-lifecycle-manager v0.0.0-20190128024246-5eb7ae5bdb7a
	github.com/rancher/rancher/pkg/apis => github.com/rancher/rancher/pkg/apis v0.0.0-20211208233239-77392a65423d
	github.com/rancher/rancher/pkg/client => github.com/rancher/rancher/pkg/client v0.0.0-20211208233239-77392a65423d
	helm.sh/helm/v3 => github.com/rancher/helm/v3 v3.7.1-rancher.1

	k8s.io/api => k8s.io/api v0.21.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.2
	k8s.io/apiserver => k8s.io/apiserver v0.21.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.21.2
	k8s.io/client-go => k8s.io/client-go v0.21.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.21.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.21.2
	k8s.io/code-generator => k8s.io/code-generator v0.21.2
	k8s.io/component-base => k8s.io/component-base v0.21.2
	k8s.io/component-helpers => k8s.io/component-helpers v0.21.2
	k8s.io/controller-manager => k8s.io/controller-manager v0.21.2
	k8s.io/cri-api => k8s.io/cri-api v0.21.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.21.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.21.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.21.2
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.21.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.21.2
	k8s.io/kubectl => k8s.io/kubectl v0.21.2
	k8s.io/kubelet => k8s.io/kubelet v0.21.2
	k8s.io/kubernetes => k8s.io/kubernetes v1.21.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.21.2
	k8s.io/metrics => k8s.io/metrics v0.21.2
	k8s.io/mount-utils => k8s.io/mount-utils v0.21.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.21.2

	kubevirt.io/client-go => github.com/kubevirt/client-go v0.45.0
	kubevirt.io/containerized-data-importer => github.com/rancher/kubevirt-containerized-data-importer v1.26.1-0.20210802100720-9bcf4e7ba0ce
	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v0.0.0-20190302045857-e85c7b244fd2
)

require (
	github.com/Jason-ZW/autok3s-geo v0.0.0-20220207061948-3abc3068deea
	github.com/alexellis/go-execute v0.0.0-20200124154445-8697e4e28c5e
	github.com/aliyun/alibaba-cloud-sdk-go v1.61.381
	github.com/aws/aws-sdk-go v1.38.65
	github.com/creack/pty v1.1.11
	github.com/docker/distribution v2.8.0+incompatible // indirect
	github.com/docker/docker v20.10.10+incompatible
	github.com/docker/go-units v0.4.0
	github.com/ghodss/yaml v1.0.0
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/harvester/harvester v1.0.0
	github.com/hpcloud/tail v1.0.0
	github.com/imdario/mergo v0.3.12
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v0.0.0-20200331171230-d50e42f2b669
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6
	github.com/morikuni/aec v1.0.0
	github.com/olekukonko/tablewriter v0.0.5
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/opencontainers/runc v1.0.3 // indirect
	github.com/rancher/apiserver v0.0.0-20211025232108-df28932a5627
	github.com/rancher/k3d/v5 v5.2.2
	github.com/rancher/k3s v1.19.5-0.20201117235738-2532c10faad4
	github.com/rancher/wrangler v0.8.10
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/tencentcloud/tencentcloud-sdk-go v1.0.34
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/net v0.0.0-20211209124913-491a49abca63
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/api v0.62.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	gorm.io/driver/sqlite v1.1.4
	gorm.io/gorm v1.20.12
	k8s.io/api v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/component-base v0.22.4
	k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	k8s.io/kubectl v0.22.3
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	kubevirt.io/client-go v0.45.0
)
