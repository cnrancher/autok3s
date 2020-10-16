package tencent

var tencentCCMTmpl = `
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: system:cloud-controller-manager
rules:
  - apiGroups:
      - ""
    resources:
      - persistentvolumes
      - services
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
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:cloud-controller-manager
subjects:
  - kind: ServiceAccount
    name: cloud-controller-manager
    namespace: kube-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cloud-controller-manager
  namespace: kube-system
---
apiVersion: v1
kind: Secret
metadata:
  name: tencentcloud-cloud-controller-manager-config
  namespace: kube-system
data:
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_REGION: "%s"
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_ID: "%s"
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_KEY: "%s"
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_VPC_ID: "%s"
  TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLUSTER_ROUTE_TABLE: "%s"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tencentcloud-cloud-controller-manager
  namespace: kube-system
spec:
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: tencentcloud-cloud-controller-manager
  template:
    metadata:
      labels:
        app: tencentcloud-cloud-controller-manager
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-role.kubernetes.io/master
                    operator: Exists
      dnsPolicy: Default
      hostNetwork: true
      serviceAccountName: cloud-controller-manager
      tolerations:
        - key: "node.cloudprovider.kubernetes.io/uninitialized"
          value: "true"
          effect: "NoSchedule"
        - key: "node.kubernetes.io/network-unavailable"
          value: "true"
          effect: "NoSchedule"
        - key: "node-role.kubernetes.io/master"
          value: "true"
          effect: "NoSchedule"
      containers:
        - image: ccr.ccs.tencentyun.com/library/tencentcloud-cloud-controller-manager:1.0.1
          name: tencentcloud-cloud-controller-manager
          command:
            - /bin/tencentcloud-cloud-controller-manager
            - --cloud-provider=tencentcloud
            - --allocate-node-cidrs=true
            - --cluster-cidr=%s
            - --configure-cloud-routes=true
            - --allow-untagged-cloud=true
            - --node-monitor-period=60s
            - --route-reconciliation-period=60s
          env:
            - name: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_REGION
              valueFrom:
                secretKeyRef:
                  name: tencentcloud-cloud-controller-manager-config
                  key: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_REGION
            - name: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_ID
              valueFrom:
                secretKeyRef:
                  name: tencentcloud-cloud-controller-manager-config
                  key: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_ID
            - name: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: tencentcloud-cloud-controller-manager-config
                  key: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_KEY
            - name: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLUSTER_ROUTE_TABLE
              valueFrom:
                secretKeyRef:
                  name: tencentcloud-cloud-controller-manager-config
                  key: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLUSTER_ROUTE_TABLE
            - name: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_VPC_ID
              valueFrom:
                secretKeyRef:
                  name: tencentcloud-cloud-controller-manager-config
                  key: TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_VPC_ID
---
`
