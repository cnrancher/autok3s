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
