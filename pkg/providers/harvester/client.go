package harvester

import (
	"fmt"
	"strings"

	"github.com/cnrancher/autok3s/pkg/utils"

	harvsterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	harvclient "github.com/harvester/harvester/pkg/generated/clientset/versioned"
	"github.com/harvester/harvester/pkg/generated/clientset/versioned/scheme"
	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

const (
	vmResource = "virtualmachines"
)

type Client struct {
	RestConfig                *rest.Config
	KubeVirtSubresourceClient *rest.RESTClient
	HarvesterClient           *harvclient.Clientset
	KubeClient                *kubernetes.Clientset
}

func NewClientFromRestConfig(restConfig *rest.Config) (*Client, error) {
	subresourceConfig := rest.CopyConfig(restConfig)
	subresourceConfig.GroupVersion = &schema.GroupVersion{Group: kubevirtv1.SubresourceGroupName, Version: kubevirtv1.ApiLatestVersion}
	subresourceConfig.APIPath = "/apis"
	subresourceConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	kubeVirtSubresourceClient, err := rest.RESTClientFor(subresourceConfig)
	if err != nil {
		return nil, err
	}
	harvClient, err := harvclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return &Client{
		RestConfig:                restConfig,
		KubeVirtSubresourceClient: kubeVirtSubresourceClient,
		HarvesterClient:           harvClient,
		KubeClient:                kubeClient,
	}, nil
}

func NamespacedNameParts(namespacedName string) (string, string, error) {
	parts := strings.Split(namespacedName, "/")
	switch len(parts) {
	case 1:
		return "", parts[0], nil
	case 2:
		return parts[0], parts[1], nil
	default:
		err := fmt.Errorf("unexpected namespacedName format (%q), expected %q or %q. ", namespacedName, "namespace/name", "name")
		return "", "", err
	}
}

func NamespacedNamePartsByDefault(namespacedName string, defaultNamespace string) (string, string, error) {
	namespace, name, err := NamespacedNameParts(namespacedName)
	if err != nil {
		return "", "", err
	}
	if namespace == "" {
		namespace = defaultNamespace
	}
	return namespace, name, nil
}

func (h *Harvester) getRestConfig() (*rest.Config, error) {
	if h.KubeConfigContent == "" {
		return kubeconfig.GetNonInteractiveClientConfig(h.KubeConfigFile).ClientConfig()
	}
	return clientcmd.RESTConfigFromKubeConfig([]byte(utils.StringSupportBase64(h.KubeConfigContent)))
}

func (h *Harvester) getClient() (*Client, error) {
	if h.client != nil {
		return h.client, nil
	}
	restConfig, err := h.getRestConfig()
	if err != nil {
		return nil, err
	}
	c, err := NewClientFromRestConfig(restConfig)
	if err != nil {
		return nil, err
	}
	h.client = c
	return h.client, nil
}

func (h *Harvester) getSetting(name string) (*harvsterv1.Setting, error) {
	c, err := h.getClient()
	if err != nil {
		return nil, err
	}
	return c.HarvesterClient.HarvesterhciV1beta1().Settings().Get(h.ctx, name, metav1.GetOptions{})
}

func (h *Harvester) getImage() (*harvsterv1.VirtualMachineImage, error) {
	c, err := h.getClient()
	if err != nil {
		return nil, err
	}
	namespace, name, err := NamespacedNamePartsByDefault(h.ImageName, h.VMNamespace)
	if err != nil {
		return nil, err
	}
	return c.HarvesterClient.HarvesterhciV1beta1().VirtualMachineImages(namespace).Get(h.ctx, name, metav1.GetOptions{})
}

func (h *Harvester) getKeyPair() (*harvsterv1.KeyPair, error) {
	c, err := h.getClient()
	if err != nil {
		return nil, err
	}
	namespace, name, err := NamespacedNamePartsByDefault(h.KeypairName, h.VMNamespace)
	if err != nil {
		return nil, err
	}
	return c.HarvesterClient.HarvesterhciV1beta1().KeyPairs(namespace).Get(h.ctx, name, metav1.GetOptions{})
}

func (h *Harvester) getNetwork() (*cniv1.NetworkAttachmentDefinition, error) {
	c, err := h.getClient()
	if err != nil {
		return nil, err
	}
	namespace, name, err := NamespacedNamePartsByDefault(h.NetworkName, h.VMNamespace)
	if err != nil {
		return nil, err
	}
	return c.HarvesterClient.K8sCniCncfIoV1().NetworkAttachmentDefinitions(namespace).Get(h.ctx, name, metav1.GetOptions{})
}

func (h *Harvester) getVMI(name string) (*kubevirtv1.VirtualMachineInstance, error) {
	c, err := h.getClient()
	if err != nil {
		return nil, err
	}
	return c.HarvesterClient.KubevirtV1().VirtualMachineInstances(h.VMNamespace).Get(h.ctx, name, metav1.GetOptions{})
}

func (h *Harvester) getVM(name string) (*kubevirtv1.VirtualMachine, error) {
	c, err := h.getClient()
	if err != nil {
		return nil, err
	}
	return c.HarvesterClient.KubevirtV1().VirtualMachines(h.VMNamespace).Get(h.ctx, name, metav1.GetOptions{})
}

func (h *Harvester) updateVM(newVM *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	c, err := h.getClient()
	if err != nil {
		return nil, err
	}
	return c.HarvesterClient.KubevirtV1().VirtualMachines(h.VMNamespace).Update(h.ctx, newVM, metav1.UpdateOptions{})
}

func (h *Harvester) deleteVM(name string) error {
	c, err := h.getClient()
	if err != nil {
		return err
	}
	propagationPolicy := metav1.DeletePropagationForeground
	return c.HarvesterClient.KubevirtV1().VirtualMachines(h.VMNamespace).Delete(h.ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
}

func (h *Harvester) putVMSubResource(name, subResource string) error {
	c, err := h.getClient()
	if err != nil {
		return err
	}
	return c.KubeVirtSubresourceClient.Put().Namespace(h.VMNamespace).Resource(vmResource).SubResource(subResource).Name(name).Do(h.ctx).Error()
}

func (h *Harvester) deleteVolume(name string) error {
	c, err := h.getClient()
	if err != nil {
		return err
	}
	return c.KubeClient.CoreV1().PersistentVolumeClaims(h.VMNamespace).Delete(h.ctx, name, metav1.DeleteOptions{})
}

func (h *Harvester) createVM(vm *kubevirtv1.VirtualMachine) (*kubevirtv1.VirtualMachine, error) {
	c, err := h.getClient()
	if err != nil {
		return nil, err
	}
	return c.HarvesterClient.KubevirtV1().VirtualMachines(h.VMNamespace).Create(h.ctx, vm, metav1.CreateOptions{})
}

func (h *Harvester) createSecret(secret *corev1.Secret) (*corev1.Secret, error) {
	c, err := h.getClient()
	if err != nil {
		return nil, err
	}
	return c.KubeClient.CoreV1().Secrets(h.VMNamespace).Create(h.ctx, secret, metav1.CreateOptions{})
}

func (h *Harvester) getVMByLabel(option string) (*kubevirtv1.VirtualMachineList, error) {
	c, err := h.getClient()
	if err != nil {
		return nil, err
	}
	return c.HarvesterClient.KubevirtV1().VirtualMachines(h.VMNamespace).List(h.ctx, metav1.ListOptions{
		LabelSelector: option,
	})
}

func (h *Harvester) getVMIByLabel(option string) (*kubevirtv1.VirtualMachineInstanceList, error) {
	c, err := h.getClient()
	if err != nil {
		return nil, err
	}
	return c.HarvesterClient.KubevirtV1().VirtualMachineInstances(h.VMNamespace).List(h.ctx, metav1.ListOptions{
		LabelSelector: option,
	})
}
