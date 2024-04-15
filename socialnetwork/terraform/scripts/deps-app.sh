#!/bin/bash

# temporarily disabled because it is getting stuck idk updating packages
#sudo apt update -y && sudo apt upgrade -y
sudo apt install -y wget git tmux python3-pip

# install Go
sudo wget https://go.dev/dl/go1.22.2.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.22.2.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
echo PATH=$PATH:/usr/local/go/bin >> ~/.bashrc
echo GOPATH=$HOME/.go >> ~/.bashrc
echo PATH=$PATH:$GOPATH/bin >> ~/.bashrc
go version

# install weaver
go install github.com/ServiceWeaver/weaver/cmd/weaver@latest
export PATH="$PATH:$HOME/go/bin"
echo PATH="$PATH:$HOME/go/bin" >> ~/.bashrc

# copy contents from gcp bucket
#export GCP_BUCKET_NAME=weaver-dsb
#gsutil -m cp -r "gs://$GCP_BUCKET_NAME/weaver-dsb-socialnetwork" .

# install requirements using venv
# machines in gcp do not allow using pip3 install etc
#python3 -m venv .venv
#source .venv/bin/activate
#cd weaver-dsb-socialnetwork
#python3 -m pip install -r requirements.txt

touch deps.ready
