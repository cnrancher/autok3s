#!/bin/bash

curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

# helm repo add pandaria http://pandaria-releases.cnrancher.com/server-charts/latest
helm repo add pandaria http://pandaria-releases.cnrancher.com/2.6-charts/dev
# helm repo add pandaria http://pandaria-releases.cnrancher.com/2.6-charts/latest
helm repo update

# no effect, just for compatibility with rancher helm template
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.7.1/cert-manager.crds.yaml

PANDARIA_VERSION=""

#docker login -u xxx -p xxx
#docker pull cnrancher/rancher:$PANDARIA_VERSION

#k3s crictl pull --creds <user>:<key> cnrancher/rancher:${PANDARIA_VERSION}

kubectl create namespace cattle-system
ec2_ip=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)
helm install rancher pandaria/rancher \
  --namespace cattle-system \
  --set hostname=$ec2_ip.sslip.io \
  --set replicas=1 \
  --set ingress.enabled=false \
  --set ingress.tls.source=rancher \
  --set bootstrapPassword=Rancher@123456 \
  --set extraEnv[0].name=CATTLE_PROMETHEUS_METRICS \
  --set-string extraEnv[0].value=true \
  --version $PANDARIA_VERSION

kubectl create -f - <<EOF
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
    port: 80
    protocol: TCP
    targetPort: 80
  - name: https
    port: 443
    protocol: TCP
    targetPort: 443
  selector:
    app: rancher
  sessionAffinity: None
  type: LoadBalancer
EOF
