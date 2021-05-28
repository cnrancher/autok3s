package cluster

const explorerTmpl = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube-explorer
  namespace: kube-system
  labels:
    app: kube-explorer
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: kube-explorer
  labels:
    app: kube-explorer
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
subjects:
- kind: ServiceAccount
  name: kube-explorer
  namespace: kube-system
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kube-explorer
  namespace: kube-system
  labels:
    app: kube-explorer
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kube-explorer
  template:
    metadata:
      namespace: kube-system
      labels:
        app: kube-explorer
    spec:
      serviceAccountName: kube-explorer
      containers:
      - image: niusmallnan/kube-explorer:v0.1.1
        imagePullPolicy: IfNotPresent
        name: kube-explorer
        ports:
        - containerPort: 8989
          protocol: TCP
        args:
        - "--https-listen-port=0"
        - "--http-listen-port=8989"
---
apiVersion: v1
kind: Service
metadata:
  name: kube-explorer
  namespace: kube-system
  labels:
    app: kube-explorer
spec:
  type: LoadBalancer
  ports:
  - port: 8989
    targetPort: 8989
    protocol: TCP
    name: http
  selector:
    app: kube-explorer
`
