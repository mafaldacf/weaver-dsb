#!/bin/bash

# temporarily disabled because it is getting stuck
#sudo apt update -y && sudo apt upgrade -y
sudo apt install -y wget git tmux python3-pip

# install Go 1.21.5
sudo wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin:$HOME/.go/bin' >> ~/.bashrc
source ~/.bashrc

# install Weaver 0.22.0
go install github.com/ServiceWeaver/weaver/cmd/weaver@v0.22.0

# copy contents from gcp bucket
#export GCP_BUCKET_NAME=weaver-dsb
#gsutil -m cp -r "gs://$GCP_BUCKET_NAME/socialnetwork" .

# install requirements using venv
# machines in gcp do not allow using pip3 install etc
#python3 -m venv .venv
#source .venv/bin/activate
#cd socialnetwork
#python3 -m pip install -r requirements.txt

touch deps.ready
