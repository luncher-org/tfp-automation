#!/bin/bash

K8S_VERSION=$1
RKE2_SERVER_IP=$2
RKE2_NEW_SERVER_IP=$3
RKE2_TOKEN=$4
CNI=$5

set -e

sudo hostnamectl set-hostname ${RKE2_NEW_SERVER_IP}

sudo mkdir -p /etc/rancher/rke2
sudo touch /etc/rancher/rke2/config.yaml

echo "server: https://${RKE2_SERVER_IP}:9345
cni: ${CNI}
token: ${RKE2_TOKEN}
tls-san:
  - ${RKE2_SERVER_IP}" | sudo tee /etc/rancher/rke2/config.yaml > /dev/null

curl -sfL https://get.rke2.io --output install.sh
sudo chmod +x install.sh

sudo INSTALL_RKE2_VERSION=${K8S_VERSION} INSTALL_RKE2_TYPE='server' sh ./install.sh

sudo systemctl enable rke2-server
sudo systemctl start rke2-server