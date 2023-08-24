package common

var DefaultRancherManifest = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: cert-manager
---
apiVersion: v1
kind: Namespace
metadata:
  name: cattle-system
---
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  namespace: kube-system
  name: cert-manager
spec:
  targetNamespace: cert-manager
  version: v1.11.0
  chart: cert-manager
  repo: https://charts.jetstack.io
  set:
    installCRDs: "true"
---
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: rancher
  namespace: kube-system
spec:
  targetNamespace: cattle-system
  repo: {{ .rancherRepo | default "https://releases.rancher.com/server-charts/latest" }}
  chart: rancher
  version: {{ .Version | default "" }}
  valuesContent: |-
    hostname: "{{ providerTemplate "public-ip-address" }}:{{ .PublicPort | default 30443 }}"
    ingress:
      enabled: false
    global:
      cattle:
        psp:
          enabled: false
    bootstrapPassword: {{ .bootstrapPassword | default "RancherForFun" }}
    antiAffinity: "required"
    replicas: 1
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: rancher
  name: rancher-lb-svc
  namespace: cattle-system
spec:
  ports:
    - name: http
      port: {{ .HTTPPort | default 30080 }}
      protocol: TCP
      targetPort: 80
    - name: https
      port: {{ .PublicPort | default 30443 }}
      protocol: TCP
      targetPort: 443
  selector:
    app: rancher
  sessionAffinity: None
  type: LoadBalancer
`
