#!/usr/bin/env bash

# Copyright © 2023 - 2024 SUSE LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

SSH_KEY=$1
if [ -z "$SSH_KEY" ]; then
	echo "You must pass a ssh key name"
	exit 1
fi

REPO_ROOT=$(git rev-parse --show-toplevel)
BASEDIR=$(dirname "$0")

RANCHER_HOSTNAME=${RANCHER_HOSTNAME:-my.hostname.dev}
RANCHER_VERSION=${RANCHER_VERSION:-v2.7.9}
RANCHER_TURTLES_VERSION=${RANCHER_TURTLES_VERSION:-v0.5.0}
CAPI_OPERATOR_VERSION=${CAPI_OPERATOR_VERSION:-v0.9.0}

export EXP_CLUSTER_RESOURCE_SET=true
export CLUSTER_TOPOLOGY=true
export SSH_KEY=${1#*=}
export CLUSTER_NAME="rke2-aws-cluster"
export AWS_REGION="us-west-2"

kind create cluster --config "$BASEDIR/kind-cluster-with-extramounts.yaml"

kubectl rollout status deployment coredns -n kube-system --timeout=90s

helm repo add jetstack https://charts.jetstack.io
helm repo add rancher-stable https://releases.rancher.com/server-charts/stable
helm repo add capi-operator https://kubernetes-sigs.github.io/cluster-api-operator
helm repo update

# Deploy cert-manager
helm install cert-manager jetstack/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --version v1.12.3 \
    --set installCRDs=true \
    --wait

kubectl rollout status deployment cert-manager -n cert-manager --timeout=90s

# Deploy rancher
helm install rancher rancher-stable/rancher \
    --namespace cattle-system \
    --create-namespace \
    --set bootstrapPassword=admin \
    --set replicas=1 \
    --set hostname=rancher.my.org \
    --set global.cattle.psp.enabled=false \
    --version="$RANCHER_VERSION" \
    --wait

kubectl rollout status deployment rancher -n cattle-system --timeout=180s

# Deploy CAPI Operator
helm install capi-operator capi-operator/cluster-api-operator \
    --create-namespace -n capi-operator-system \
    --set cert-manager.enabled=false \
    --set cluster-api-operator.cluster-api.enabled=false \
    --version="$CAPI_OPERATOR_VERSION" \
    --wait

kubectl rollout status deployment capi-operator-cluster-api-operator -n capi-operator-system --timeout=180s

helm repo add turtles https://rancher-sandbox.github.io/rancher-turtles/
helm repo update

# Deploy Rancher Turtles extension
helm install rancher-turtles turtles/rancher-turtles \
    --version="$RANCHER_TURTLES_VERSION" \
    --create-namespace \
    -n rancher-turtles-system \
    --set cluster-api-operator.cert-manager.enabled=false \
    --set cluster-api-operator.enabled=false \
    --dependency-update \
    --wait

kubectl rollout status deployment rancher-turtles-controller-manager -n rancher-turtles-system --timeout=180s

cd ${REPO_ROOT}/examples

# Deploy RKE2 provider
kubectl apply -f rke2-provider.yaml
sleep 60

kubectl rollout status deployment capi-controller-manager -n capi-system --timeout=180s
kubectl rollout status deployment capi-kubeadm-bootstrap-controller-manager -n capi-kubeadm-bootstrap-system --timeout=180s
kubectl rollout status deployment capi-kubeadm-control-plane-controller-manager -n capi-kubeadm-control-plane-system --timeout=180s
kubectl rollout status deployment rke2-bootstrap-controller-manager -n rke2-bootstrap-system --timeout=180s
kubectl rollout status deployment rke2-control-plane-controller-manager -n rke2-control-plane-system --timeout=180s

# Deploy AWS provider
export AWS_B64ENCODED_CREDENTIALS=$(clusterawsadm bootstrap credentials encode-as-profile)
envsubst < aws-provider.yaml > aws-provider-applied.yaml
kubectl apply -f aws-provider-applied.yaml
sleep 20

kubectl rollout status deployment capa-controller-manager -n capa-system --timeout=180s

# Deploy RKE2 Cluster in AWS
envsubst < rke2-aws-cluster.yaml > rke2-aws-cluster-applied.yaml
kubectl apply -f rke2-aws-cluster-applied.yaml

cd ${REPO_ROOT}

# Tilt up
tilt up