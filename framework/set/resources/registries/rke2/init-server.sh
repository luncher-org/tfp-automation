#!/bin/bash

USER=$1
GROUP=$2
K8S_VERSION=$3
RKE2_SERVER_IP=$4
RKE2_TOKEN=$5
RANCHER_IMAGE=$6
RANCHER_TAG_VERSION=$7
REGISTRY=$8
RANCHER_AGENT_IMAGE=${9}

set -e

sudo mkdir -p /etc/rancher/rke2
sudo touch /etc/rancher/rke2/config.yaml

echo "token: ${RKE2_TOKEN}
tls-san:
  - ${RKE2_SERVER_IP}" | sudo tee /etc/rancher/rke2/config.yaml > /dev/null

sudo tee -a /etc/rancher/rke2/registries.yaml > /dev/null << EOF
mirrors:
  docker.io:
    endpoint:
      - "https://${REGISTRY}"
configs:
  "${REGISTRY}":
    tls:
      insecure_skip_verify: true
EOF

ARCH=$(uname -m)
if [[ $ARCH == "x86_64" ]]; then
    ARCH="amd64"
elif [[ $ARCH == "arm64" || $ARCH == "aarch64" ]]; then
    ARCH="arm64"
fi

wget https://github.com/rancher/rke2/releases/download/${K8S_VERSION}+rke2r1/rke2.linux-${ARCH}.tar.gz
wget https://github.com/rancher/rke2/releases/download/${K8S_VERSION}+rke2r1/rke2.linux-${ARCH}.tar.gz
wget https://github.com/rancher/rke2/releases/download/${K8S_VERSION}+rke2r1/rke2-images.linux-${ARCH}.tar.zst
wget https://github.com/rancher/rke2/releases/download/${K8S_VERSION}+rke2r1/rke2-images.linux-${ARCH}.tar.zst
wget https://github.com/rancher/rke2/releases/download/${K8S_VERSION}+rke2r1/sha256sum-${ARCH}.txt
wget https://github.com/rancher/rke2/releases/download/${K8S_VERSION}+rke2r1/sha256sum-${ARCH}.txt

curl -sfL https://get.rke2.io --output install.sh
sudo chmod +x install.sh

sudo INSTALL_RKE2_ARTIFACT_PATH=/home/${USER} sh install.sh
sudo systemctl enable rke2-server
sudo systemctl start rke2-server

sudo tee /etc/docker/daemon.json > /dev/null << EOF
{
  "insecure-registries" : [ "${REGISTRY}" ]
}
EOF

sudo systemctl restart docker && sudo systemctl daemon-reload

if [ -n "$RANCHER_AGENT_IMAGE" ]; then
  sudo docker pull ${REGISTRY}/${RANCHER_IMAGE}:${RANCHER_TAG_VERSION}
  sudo docker pull ${REGISTRY}/${RANCHER_AGENT_IMAGE}:${RANCHER_TAG_VERSION}
  sudo systemctl restart rke2-server
fi

sudo mkdir -p /home/${USER}/.kube
sudo cp /etc/rancher/rke2/rke2.yaml /home/${USER}/.kube/config
sudo chown -R ${USER}:${GROUP} /home/${USER}/.kube