package alibaba

// See: https://github.com/kubernetes/cloud-provider-alibaba-cloud/blob/master/deploy/v2/cloud-controller-manager.yaml.
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
  cloud-config.conf: |-
    {
        "Global": {
            "accessKeyID": "%s",
            "accessKeySecret": "%s"
        }
    }
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:cloud-controller-manager
  labels: 
    app: "alibaba-ccm"
rules:
  - apiGroups:
      - ""
    resources:
      - events
    verbs:
      - create
      - patch
      - update
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
      - nodes/status
    verbs:
      - patch
      - update
  - apiGroups:
      - ""
    resources:
      - services
    verbs:
      - get
      - list
      - watch
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - services/status
    verbs:
      - update
      - patch
  - apiGroups:
      - ""
    resources:
      - serviceaccounts
    verbs:
      - create
  - apiGroups:
      - ""
    resources:
      - endpoints
    verbs:
      - get
      - list
      - watch
      - create
      - patch
      - update
  - apiGroups:
      - coordination.k8s.io
    resources:
      - leases
    verbs:
      - get
      - list
      - update
      - create
  - apiGroups:
      - apiextensions.k8s.io
    resources:
      - customresourcedefinitions
    verbs:
      - get
      - update
      - create
      - delete
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cloud-controller-manager
  namespace: kube-system
  labels: 
    app: "alibaba-ccm"
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
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
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    app: "alibaba-ccm"
  name: cloud-controller-manager
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: "alibaba-ccm"
  template:
    metadata:
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ""
      labels:
        app: "alibaba-ccm"
    spec:
      containers:
        - command:
            - /cloud-controller-manager
            - --kubeconfig=/etc/kubernetes/k3s.yaml
            - --cloud-config=/etc/kubernetes/config/cloud-config.conf
            - --metrics-bind-addr=0
            - --allocate-node-cidrs=true
            - --cluster-cidr=%s
          image: registry.%s.aliyuncs.com/acs/cloud-controller-manager-amd64:v2.4.0
          imagePullPolicy: IfNotPresent
          livenessProbe:
            failureThreshold: 8
            httpGet:
              host: 127.0.0.1
              path: /healthz
              port: 10258
              scheme: HTTP
            initialDelaySeconds: 15
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 15
          name: cloud-controller-manager
          resources:
            limits:
              cpu: "1"
              memory: 1Gi
            requests:
              cpu: 100m
              memory: 200Mi
          volumeMounts:
            - mountPath: /etc/kubernetes/k3s.yaml
              name: k8s
              readOnly: true
            - mountPath: /etc/kubernetes/config
              name: cloud-config
      hostNetwork: true
      nodeSelector:
        node-role.kubernetes.io/master: "true"
      restartPolicy: Always
      serviceAccountName: cloud-controller-manager
      tolerations:  
        - effect: NoSchedule  
          operator: Exists  
          key: node-role.kubernetes.io/master 
        - effect: NoSchedule  
          operator: Exists  
          key: node.cloudprovider.kubernetes.io/uninitialized
      volumes:
        - hostPath:
            path: /etc/rancher/k3s/k3s.yaml
            type: File
          name: k8s
        - configMap:
            defaultMode: 420
            items:
              - key: cloud-config.conf
                path: cloud-config.conf
            name: cloud-config
          name: cloud-config
  updateStrategy:
    rollingUpdate:
      maxSurge: 0
      maxUnavailable: 1
    type: RollingUpdate
`
