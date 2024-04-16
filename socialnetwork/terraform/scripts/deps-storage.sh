#!/bin/bash

# temporarily disabled because it is getting stuck idk updating packages
#sudo apt update -y && sudo apt upgrade -y
sudo apt install -y docker.io docker-compose dnsutils curl wget

# copy contents from gcp bucket
export GCP_BUCKET_NAME=weaver-dsb
gsutil -m cp -r "gs://$GCP_BUCKET_NAME/socialnetwork" .

# build images
sudo docker build -t mongodb-delayed:4.4.6 /socialnetwork/docker/mongodb-delayed/.
sudo docker build -t mongodb-setup:4.4.6 /socialnetwork/docker/mongodb-setup/post-storage/.
sudo docker build -t rabbitmq-setup:3.8 /socialnetwork/docker/rabbitmq-setup/write-home-timeline/.

touch deps.ready
