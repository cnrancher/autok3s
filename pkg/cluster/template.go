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
