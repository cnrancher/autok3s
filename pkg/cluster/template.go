package cluster

const dashboardTmpl = `
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: dashboard
  namespace: kube-system
spec:
  chart: kubernetes-dashboard
  repo: %s
  targetNamespace: kube-system
  valuesContent: |-
    service:
      type: LoadBalancer
      externalPort: 8999
      internalPort: 8999
`

const octopusTmpl = `
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: octopus-ui
  namespace: kube-system
spec:
  chart: octopus-ui
  repo: http://charts.cnrancher.com/octopus
  targetNamespace: octopus-system
  valuesContent: |-
    service:
      type: LoadBalancer
      port: 8999
`

const alibabaCCMTmpl = `
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: alibaba-ccm
  namespace: kube-system
spec:
  chart: alibaba-ccm
  repo: http://charts.cnrancher.com/autok3s
  targetNamespace: kube-system
  valuesContent: |-
    accessKey: %s
    accessSecret: %s
    clusterCIDR: %s
    rbac:
      create: true
    serviceAccount:
      create: true
`

const terwayTmpl = `
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: terway
  namespace: kube-system
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: terway-pod-reader
  namespace: kube-system
rules:
- apiGroups: [\"\"]
  resources: [\"pods\", \"nodes\", \"namespaces\", \"configmaps\", \"serviceaccounts\"]
  verbs: [\"get\", \"watch\", \"list\", \"update\"]
- apiGroups: [\"\"]
  resources:
    - events
  verbs:
    - create
- apiGroups: [\"networking.k8s.io\"]
  resources:
  - networkpolicies
  verbs:
  - get
  - list
  - watch
- apiGroups: [\"extensions\"]
  resources:
  - networkpolicies
  verbs:
  - get
  - list
  - watch
- apiGroups: [\"\"]
  resources:
  - pods/status
  verbs:
  - update
- apiGroups: [\"crd.projectcalico.org\"]
  resources: [\"*\"]
  verbs: [\"*\"]
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: terway-binding
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: terway-pod-reader
subjects:
- kind: ServiceAccount
  name: terway
  namespace: kube-system
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: eni-config
  namespace: kube-system
data:
  eni_conf: |
    {
      \"version\": \"1\",
      \"access_key\": \"%s\",
      \"access_secret\": \"%s\",
      \"security_group\": \"%s\",
      \"service_cidr\": \"%s\",
      \"vswitches\": %s,
      \"max_pool_size\": %s,
      \"min_pool_size\": 0
    }
  10-terway.conf: |
    {
      \"cniVersion\": \"0.3.0\",
      \"name\": \"terway\",
      \"type\": \"terway\",
      \"eniip_virtual_type\": \"Veth\"
    }
  disable_network_policy: \"false\"
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: terway
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: terway
  template:
    metadata:
      labels:
        app: terway
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      hostPID: true
      nodeSelector:
        beta.kubernetes.io/arch: amd64
      tolerations:
      - operator: \"Exists\"
      terminationGracePeriodSeconds: 0
      serviceAccountName: terway
      hostNetwork: true
      initContainers:
      - name: terway-cni-installer
        image: zhenyangzhao/cni-installer:v0.8.6
        imagePullPolicy: IfNotPresent
        securityContext:
          privileged: true
        command:
          - 'sh'
          - '-c'
          - 'cp -r /opt/cni-installer/bin/* /opt/cni/bin/; chmod a+x /opt/cni/bin/;'
        volumeMounts:
          - name: cni-bin
            mountPath: /opt/cni/bin
      - name: terway-init
        image: registry.aliyuncs.com/acs/terway:v1.0.10.122-gd0be015-aliyun
        imagePullPolicy: IfNotPresent
        securityContext:
          privileged: true
        command:
        - 'sh'
        - '-c'
        - 'cp /usr/bin/terway /opt/cni/bin/;
                  chmod +x /opt/cni/bin/terway;
                  cp /etc/eni/10-terway.conf /etc/cni/net.d/;
                  sysctl -w net.ipv4.conf.eth0.rp_filter=0;
                  modprobe sch_htb || true;
                  chroot /host sh -c \"systemctl disable eni.service; rm -f /etc/udev/rules.d/75-persistent-net-generator.rules /lib/udev/rules.d/60-net.rules /lib/udev/rules.d/61-eni.rules /lib/udev/write_net_rules && udevadm control --reload-rules && udevadm trigger; true\"'
        volumeMounts:
        - name: configvolume
          mountPath: /etc/eni
        - name: cni-bin
          mountPath: /opt/cni/bin/
        - name: cni
          mountPath: /etc/cni/net.d/
        - mountPath: /lib/modules
          name: lib-modules
        - mountPath: /host
          name: host-root
      containers:
      - name: terway
        image: registry.aliyuncs.com/acs/terway:v1.0.10.122-gd0be015-aliyun
        imagePullPolicy: IfNotPresent
        command: ['/usr/bin/terwayd', '-log-level', 'debug', '-daemon-mode', 'ENIMultiIP']
        securityContext:
          privileged: true
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: POD_NAMESPACE
          valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
        volumeMounts:
        - name: configvolume
          mountPath: /etc/eni
        - mountPath: /var/run/
          name: eni-run
        - mountPath: /opt/cni/bin/
          name: cni-bin
        - mountPath: /lib/modules
          name: lib-modules
        - mountPath: /var/lib/cni/networks
          name: cni-networks
        - mountPath: /var/lib/cni/terway
          name: cni-terway
        - mountPath: /var/lib/kubelet/device-plugins
          name: device-plugin-path
      - name: policy
        image: registry.aliyuncs.com/acs/terway:v1.0.10.122-gd0be015-aliyun
        imagePullPolicy: IfNotPresent
        command: [\"/bin/policyinit.sh\"]
        env:
        - name: NODENAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: DISABLE_POLICY
          valueFrom:
            configMapKeyRef:
              name: eni-config
              key: disable_network_policy
              optional: true
        securityContext:
          privileged: true
        resources:
          requests:
            cpu: 250m
        livenessProbe:
          tcpSocket:
            port: 9099
            host: localhost
          periodSeconds: 10
          initialDelaySeconds: 10
          failureThreshold: 6
        readinessProbe:
          tcpSocket:
            port: 9099
            host: localhost
          periodSeconds: 10
        volumeMounts:
        - mountPath: /lib/modules
          name: lib-modules
      volumes:
      - name: configvolume
        configMap:
          name: eni-config
          items:
          - key: eni_conf
            path: eni.json
          - key: 10-terway.conf
            path: 10-terway.conf
      - name: cni-bin
        hostPath:
          path: /opt/cni/bin
          type: \"DirectoryOrCreate\"
      - name: cni
        hostPath:
          path: /etc/cni/net.d
      - name: eni-run
        hostPath:
          path: /var/run/
          type: \"Directory\"
      - name: lib-modules
        hostPath:
          path: /lib/modules
      - name: cni-networks
        hostPath:
          path: /var/lib/cni/networks
      - name: cni-terway
        hostPath:
          path: /var/lib/cni/terway
      - name: device-plugin-path
        hostPath:
          path: /var/lib/kubelet/device-plugins
          type: \"Directory\"
      - name: host-root
        hostPath:
          path: /
          type: \"Directory\"
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: felixconfigurations.crd.projectcalico.org
spec:
  scope: Cluster
  group: crd.projectcalico.org
  version: v1
  names:
    kind: FelixConfiguration
    plural: felixconfigurations
    singular: felixconfiguration
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: bgpconfigurations.crd.projectcalico.org
spec:
  scope: Cluster
  group: crd.projectcalico.org
  version: v1
  names:
    kind: BGPConfiguration
    plural: bgpconfigurations
    singular: bgpconfiguration
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: ippools.crd.projectcalico.org
spec:
  scope: Cluster
  group: crd.projectcalico.org
  version: v1
  names:
    kind: IPPool
    plural: ippools
    singular: ippool
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: hostendpoints.crd.projectcalico.org
spec:
  scope: Cluster
  group: crd.projectcalico.org
  version: v1
  names:
    kind: HostEndpoint
    plural: hostendpoints
    singular: hostendpoint
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: clusterinformations.crd.projectcalico.org
spec:
  scope: Cluster
  group: crd.projectcalico.org
  version: v1
  names:
    kind: ClusterInformation
    plural: clusterinformations
    singular: clusterinformation
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: globalnetworkpolicies.crd.projectcalico.org
spec:
  scope: Cluster
  group: crd.projectcalico.org
  version: v1
  names:
    kind: GlobalNetworkPolicy
    plural: globalnetworkpolicies
    singular: globalnetworkpolicy
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: globalnetworksets.crd.projectcalico.org
spec:
  scope: Cluster
  group: crd.projectcalico.org
  version: v1
  names:
    kind: GlobalNetworkSet
    plural: globalnetworksets
    singular: globalnetworkset
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: networkpolicies.crd.projectcalico.org
spec:
  scope: Namespaced
  group: crd.projectcalico.org
  version: v1
  names:
    kind: NetworkPolicy
    plural: networkpolicies
    singular: networkpolicy
`
