package alibaba

const alibabaCCMTmpl = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cloud-config
  namespace: kube-system
  labels:
    app: "alibaba-ccm"
data:
  special.keyid: "%s"
  special.keysecret: "%s"
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: system:cloud-controller-manager
  namespace: kube-system
  labels:
    app: "alibaba-ccm"
rules:
  - apiGroups:
      - ""
    resources:
      - persistentvolumes
      - services
      - secrets
      - endpoints
      - serviceaccounts
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - nodes
    verbs:
      - get
      - list
      - watch
      - delete
      - patch
      - update
  - apiGroups:
      - ""
    resources:
      - services/status
    verbs:
      - update
  - apiGroups:
      - ""
    resources:
      - nodes/status
    verbs:
      - patch
      - update
  - apiGroups:
      - ""
    resources:
      - events
      - endpoints
    verbs:
      - create
      - patch
      - update
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: system:cloud-controller-manager
  labels:
    app: "alibaba-ccm"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:cloud-controller-manager
subjects:
  - kind: ServiceAccount
    name: cloud-controller-manager
    namespace: kube-system
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: system:shared-informers
  labels:
    app: "alibaba-ccm"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:cloud-controller-manager
subjects:
  - kind: ServiceAccount
    name: shared-informers
    namespace: kube-system
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: system:cloud-node-controller
  labels:
    app: "alibaba-ccm"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:cloud-controller-manager
subjects:
  - kind: ServiceAccount
    name: cloud-node-controller
    namespace: kube-system
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: system:pvl-controller
  labels:
    app: "alibaba-ccm"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:cloud-controller-manager
subjects:
  - kind: ServiceAccount
    name: pvl-controller
    namespace: kube-system
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: system:route-controller
  labels:
    app: "alibaba-ccm"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:cloud-controller-manager
subjects:
  - kind: ServiceAccount
    name: route-controller
    namespace: kube-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cloud-controller-manager
  namespace: kube-system
  labels:
    app: "alibaba-ccm"
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    app: "alibaba-ccm"
    tier: control-plane
  name: cloud-controller-manager
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: "alibaba-ccm"
      tier: control-plane
  template:
    metadata:
      labels:
        app: "alibaba-ccm"
        tier: control-plane
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      serviceAccountName: cloud-controller-manager
      tolerations:
        - effect: NoSchedule
          operator: Exists
          key: node-role.kubernetes.io/master
        - effect: NoSchedule
          operator: Exists
          key: node.cloudprovider.kubernetes.io/uninitialized
      nodeSelector:
        node-role.kubernetes.io/master: "true"
      containers:
        - command:
            -  /cloud-controller-manager
            - --kubeconfig=/etc/kubernetes/k3s.yaml
            - --address=127.0.0.1
            - --allow-untagged-cloud=true
            - --leader-elect=true
            - --cloud-provider=alicloud
            - --allocate-node-cidrs=true
            - --cluster-cidr=%s
            - --use-service-account-credentials=true
            - --route-reconciliation-period=30s
            - --v=5
          image: registry.%s.aliyuncs.com/acs/cloud-controller-manager-amd64:v1.9.3.239-g40d97e1-aliyun
          env:
            - name: ACCESS_KEY_ID
              valueFrom:
                configMapKeyRef:
                  name: cloud-config
                  key: special.keyid
            - name: ACCESS_KEY_SECRET
              valueFrom:
                configMapKeyRef:
                  name: cloud-config
                  key: special.keysecret
          livenessProbe:
            failureThreshold: 8
            httpGet:
              host: 127.0.0.1
              path: /healthz
              port: 10252
              scheme: HTTP
            initialDelaySeconds: 15
            timeoutSeconds: 15
          name: cloud-controller-manager
          resources:
            requests:
              cpu: 200m
          volumeMounts:
            - mountPath: /etc/kubernetes/
              name: k8s
              readOnly: true
            - mountPath: /etc/ssl/certs
              name: certs
            - mountPath: /etc/pki
              name: pki
      hostNetwork: true
      volumes:
        - hostPath:
            path: /etc/rancher/k3s
          name: k8s
        - hostPath:
            path: /etc/ssl/certs
          name: certs
        - hostPath:
            path: /etc/pki
          name: pki
`
