#!/bin/bash

curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

helm repo add rancher-latest https://releases.rancher.com/server-charts/latest
helm repo add jetstack https://charts.jetstack.io
helm repo update

helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set installCRDs=true \
  --set image.repository=cnrancher/mirrored-jetstack-cert-manager-controller \
  --set webhook.image.repository=cnrancher/mirrored-jetstack-cert-manager-webhook \
  --set cainjector.image.repository=cnrancher/mirrored-jetstack-cert-manager-cainjector \
  --set startupapicheck.image.repository=cnrancher/mirrored-jetstack-cert-manager-ctl \
  --version v1.10.1

kubectl create namespace cattle-system
ec2_ip=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)
helm install rancher rancher-latest/rancher \
  --namespace cattle-system \
  --set hostname=$ec2_ip.sslip.io \
  --set replicas=1 \
  --set bootstrapPassword=Rancher@123456 \
  --set extraEnv[0].name=CATTLE_PROMETHEUS_METRICS \
  --set-string extraEnv[0].value=true
