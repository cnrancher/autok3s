package harvester

import (
	"fmt"
	"io/ioutil"

	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/ghodss/yaml"
	"github.com/harvester/harvester/pkg/builder"
	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	userDataHeader = `#cloud-config
`
	userDataAddQemuGuestAgent = `
package_update: true
packages:
- qemu-guest-agent
runcmd:
- [systemctl, enable, --now, qemu-guest-agent]`
	userDataPasswordTemplate = `
user: %s
password: %s
chpasswd: { expire: False }
ssh_pwauth: True`

	userDataSSHKeyTemplate = `
ssh_authorized_keys:
- >-
  %s`
)

func (h *Harvester) buildCloudInit(name string) (*builder.CloudInitSource, *corev1.Secret, error) {
	cloudInitSource := &builder.CloudInitSource{
		CloudInitType: builder.CloudInitTypeNoCloud,
	}
	userData, networkData, err := h.mergeCloudInit()
	if err != nil {
		return nil, nil, err
	}
	cloudConfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", name, "cloudinit"),
			Namespace: h.VMNamespace,
		},
		Data: map[string][]byte{},
	}
	if userData != "" {
		cloudConfigSecret.Data["userdata"] = []byte(userData)
		cloudInitSource.UserDataSecretName = cloudConfigSecret.Name
	}
	if networkData != "" {
		cloudConfigSecret.Data["networkdata"] = []byte(networkData)
		cloudInitSource.NetworkDataSecretName = cloudConfigSecret.Name
	}
	if len(cloudConfigSecret.Data) == 0 {
		cloudConfigSecret = nil
	}
	return cloudInitSource, cloudConfigSecret, nil
}

func (h *Harvester) mergeCloudInit() (string, string, error) {
	var (
		userData    string
		networkData string
	)
	// userData
	if h.NetworkType != networkTypePod {
		// need qemu guest agent to get ip
		userData += userDataAddQemuGuestAgent
	}
	if h.SSHPassword != "" {
		userData += fmt.Sprintf(userDataPasswordTemplate, h.SSHUser, h.SSHPassword)
	}
	if h.SSHPublicKey != "" {
		userData += fmt.Sprintf(userDataSSHKeyTemplate, h.SSHPublicKey)
	}
	if h.CloudConfig != "" {
		cloudConfigContent, err := ioutil.ReadFile(h.CloudConfig)
		if err != nil {
			return "", "", err
		}
		userDataByte, err := mergeYaml([]byte(userData), cloudConfigContent)
		if err != nil {
			return "", "", err
		}
		userData = string(userDataByte)
	}
	if h.UserData != "" {
		userDataByte, err := mergeYaml([]byte(userData), []byte(utils.StringSupportBase64(h.UserData)))
		if err != nil {
			return "", "", err
		}
		userData = string(userDataByte)
	}
	userData = userDataHeader + userData
	if h.NetworkData != "" {
		networkData = utils.StringSupportBase64(h.NetworkData)
	}
	return userData, networkData, nil
}

func mergeYaml(dst, src []byte) ([]byte, error) {
	var (
		srcData = make(map[string]interface{})
		dstData = make(map[string]interface{})
	)
	if err := yaml.Unmarshal(src, &srcData); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(dst, &dstData); err != nil {
		return nil, err
	}
	if err := mergo.Map(&dstData, srcData, mergo.WithAppendSlice); err != nil {
		return nil, err
	}
	return yaml.Marshal(dstData)
}
