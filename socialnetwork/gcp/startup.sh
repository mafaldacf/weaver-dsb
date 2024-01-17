sudo apt update -y && sudo apt upgrade -y
sudo apt install -y docker.io docker-compose dnsutils curl wget

#gcloud auth activate-service-account --key-file=credentials.json
gsutil cp -r gs://weaver-411410_cloudbuild/weaver-dsb-socialnetwork . 
sudo docker build -t mongodb-delayed:4.4.6 weaver-dsb-socialnetwork/docker/mongodb-delayed/.
sudo docker build -t mongodb-setup:4.4.6 weaver-dsb-socialnetwork/docker/mongodb-setup/post-storage/.
sudo docker build -t rabbitmq-setup:3.8 weaver-dsb-socialnetwork/docker/rabbitmq-setup/write-home-timeline/.
