#!/bin/bash

# temporarily disabled because it is getting stuck
#sudo apt update -y && sudo apt upgrade -y
sudo apt install -y wget git tmux python3-pip rsync

# install Go 1.21.5
sudo wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin:$HOME/.go/bin' >> ~/.bashrc
source ~/.bashrc

# install Weaver 0.22.0
go install github.com/ServiceWeaver/weaver/cmd/weaver@v0.22.0

touch deps.ready
