# DeathStarBench SocialNetwork @ Service Weaver

Implementation of [DeathStarBench](https://github.com/delimitrou/DeathStarBench) SocialNetwork service using Service Weaver framework, drawing inspiration from the [Blueprint](https://gitlab.mpi-sws.org/cld/blueprint)'s repository.

# Table of Contents
- [DeathStarBench SocialNetwork @ Service Weaver](#deathstarbench-socialnetwork--service-weaver)
- [Table of Contents](#table-of-contents)
- [1. Requirements](#1-requirements)
- [2. Configuration](#2-configuration)
  - [2.1. GCP Configuration](#21-gcp-configuration)
  - [2.2. Workload Configuration](#22-workload-configuration)
  - [2.3. Docker Configuration](#23-docker-configuration)
- [3. Application Deployment](#3-application-deployment)
  - [3.1. Local Deployment using Weaver Multi Process](#31-local-deployment-using-weaver-multi-process)
  - [3.2. GCP Deployment](#32-gcp-deployment)
- [4. Complementary Information](#4-complementary-information)
  - [4.1. Manually Testing HTTP Workload Generator](#41-manually-testing-http-workload-generator)
  - [4.2. Manually Testing HTTP Requests](#42-manually-testing-http-requests)

# 1. Requirements

- [Docker >= v26.1.4](https://docs.docker.com/engine/install)
- [Docker Compose >= v2.27.1](https://docs.docker.com/compose/install)
- [Python >= v3.12.3](https://www.python.org/downloads/)
- [Terraform >= v1.6.6](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli)
- [Ansible >= v2.15.2](https://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html)
- [GCloud Cli](https://cloud.google.com/sdk/docs/install)
- [Golang >= 1.21.5](https://go.dev/doc/install)
- [Service Weaver >= v0.22.0](https://serviceweaver.dev/docs.html#installation)

Install python packages to use the `manager.py` script:
```zsh
pip install -r requirements.txt
```

[**OPTIONAL**] if HTTP Workload Generator (wrk2) is going to run locally
- `Lua >= 5.1.5`
- `LuaRocks >= 3.8.0` (with `lua-json`, `luasocket`, `penlight` packages)
- `OpenSSL >= 3.0.2`

```zsh
sudo apt-get install -y libssl-dev luarocks
sudo luarocks install lua-json
sudo luarocks install luasocket
sudo luarocks install penlight
```

# 2. Configuration

## 2.1. GCP Configuration

1. Ensure that you have a GCP project created and setup as your default project in `gcloud cli`:
``` zsh
# initialize gcloud (set default region e.g. as europe-west3-a)
gcloud init
# list all projects
gcloud projects list
# select the desired project id
gcloud config set project YOUR_PROJECT_ID
# verify it is now set as default
gcloud config get-value project
```
1. Ensure that [Compute Engine API](https://console.cloud.google.com/marketplace/product/google/compute.googleapis.com) is enabled in GCP
2. Go to `weaver-dsb/socialnetwork/gcp/config.yml` and place you GCP `project_id` and any desired `username` for accessing GCP machines using that hostname
3. Configure GCP firewalls and SSH keys for Compute Engine
``` zsh
./manager.py --gcp configure
```
4. Setup a new Service Account key for authenticating (for more information: https://developers.google.com/workspace/guides/create-credentials)
    - Go to [IAM & Admin -> Service Accounts](https://console.cloud.google.com/iam-admin/serviceaccounts) of your project
    - Select your compute engine default service account
    - Go to the keys tab and select `ADD KEY` to create a new key in JSON
    - Place your JSON file as `credentials.json` in `weaver-dsb/socialnetwork/gcp/credentials.json`

## 2.2. Workload Configuration

Generate workload binary:

```zsh
cd wrk2
make
```

## 2.3. Docker Configuration

The following command builds a docker image that can be used to run the `manager.py` script in a containerized way instead of using your local environment
``` zsh
docker build -t weaver-dsb-sn .
```

Then, use the flag `--docker` to run the script. E.g. `./manager.py --docker --local build`, `./manager.py --docker --gcp deploy`, etc.

# 3. Application Deployment

## 3.1. Local Deployment using Weaver Multi Process

Build docker images and deploy datastores (mongodb, redis, rabbitmq):
``` zsh
./manager.py --local build
./manager.py --local run
```

Deploy and run application:

``` zsh
go build
weaver multi deploy weaver-local.toml
```

[**OPTIONAL**] Init social graph (not necessary if workload only runs sequences of `ComposePost`, which the default for now):

``` zsh
./manager.py --local init-social-graph
```

Run workload and automatically gather metrics to `evaluation` directory. If not specified, the default parameters are 2 threads, 2 clients, 30 duration (in seconds), 50 rate
``` zsh
./manager.py --local wrk2 -t THREADS -c CLIENTS -d DURATION -r RATE
./manager.py --local wrk2
```

Stop datastores:
``` zsh
./manager.py --local stop
```

## 3.2. GCP Deployment

Use the following commands to deploy and start the application in GCP machines and display info for docker swarm and hosts of GCP machines:
``` zsh
./manager.py --gcp deploy
./manager.py --gcp start
./manager.py --gcp info
```

[**OPTIONAL**] Init social graph (not necessary if workload only runs sequences of `ComposePost`, which the default for now):

``` zsh
./manager.py --gcp init-social-graph
```

Run workload and automatically gather metrics to `evaluation` directory. If not specified, the default parameters are 2 threads, 2 clients, 30 duration (in seconds), 50 rate
``` zsh
./manager.py --gcp wrk2 -t THREADS -c CLIENTS -d DURATION -r RATE
./manager.py --gcp wrk2
```

Restart datastores and application:
``` zsh
./manager.py --gcp restart
```

Otherwise, to clean all gcp resources at the end, do:

``` zsh
./manager.py --gcp clean
```

# 4. Complementary Information

## 4.1. Manually Testing HTTP Workload Generator

Compose Posts

```zsh
cd wrk2
./wrk -D exp -t <num-threads> -c <num-conns> -d <duration> -L -s ./scripts/social-network/compose-post.lua http://localhost:9000/wrk2-api/post/compose -R <reqs-per-sec>
# e.g.
./wrk -D exp -t 1 -c 1 -d 1 -L -s ./scripts/social-network/compose-post.lua http://localhost:9000/wrk2-api/post/compose -R 1
```

Read Home Timelines

```zsh
cd wrk2
./wrk -D exp -t <num-threads> -c <num-conns> -d <duration> -L -s ./scripts/social-network/read-home-timeline.lua http://localhost:9000/wrk2-api/home-timeline/read -R <reqs-per-sec>
```

Read User Timelines

```zsh
cd wrk2
./wrk -D exp -t <num-threads> -c <num-conns> -d <duration> -L -s ./scripts/social-network/read-user-timeline.lua http://localhost:9000/wrk2-api/user-timeline/read -R <reqs-per-sec>
```

## 4.2. Manually Testing HTTP Requests

**Register User**: {username, first_name, last_name, password} [user_id]

``` zsh
curl -X POST "localhost:9000/wrk2-api/user/register" -d "username=USERNAME&user_id=USER_ID&first_name=FIRST_NAME&last_name=LAST_NAME&password=PASSWORD"
# e.g.
curl -X POST "localhost:9000/wrk2-api/user/register" -d "username=ana&user_id=0&first_name=ana1&last_name=ana2&password=123"
curl -X POST "localhost:9000/wrk2-api/user/register" -d "username=bob&user_id=1&first_name=bob1&last_name=bob2&password=123"
```

**Follow User**: [{user_id, followee_id}, {user_name, followee_name}]

``` zsh
curl -X POST "localhost:9000/wrk2-api/user/follow" -d "user_id=USER_ID&followee_id=FOLLOWEE_ID"
# OR ALTERNATIVELY
curl -X POST "localhost:9000/wrk2-api/user/follow" -d "user_name=USER_NAME&followee_name=FOLLOWEE_AME"

# e.g.
curl -X POST "localhost:9000/wrk2-api/user/follow" -d "user_name=ana&followee_name=bob"
curl -X POST "localhost:9000/wrk2-api/user/follow" -d "user_id=1&followee_id=0"
```

**Unfollow User**: [{user_id, followee_id}, {username, followee_name}]

``` zsh
curl -X POST "localhost:9000/wrk2-api/user/unfollow" -d "user_id=USER_ID&followee_id=FOLLOWEE_ID"
# e.g.
curl -X POST "localhost:9000/wrk2-api/user/unfollow" -d "user_id=1&followee_id=0"
```

**Compose Post**: {user_id, text, username, post_type} [media_types, media_ids]

``` zsh
curl -X POST "localhost:9000/wrk2-api/post/compose" -d "user_id=USER_ID&text=TEXT&username=USER_ID&post_type=POST_TYPE"
# e.g.
curl -X POST "localhost:9000/wrk2-api/post/compose" -d "user_id=0&text=helloworld_0&username=ana&post_type=0&media_types=["png"]&media_ids=[0]"
curl -X POST "localhost:9000/wrk2-api/post/compose" -d "user_id=1&text=helloworld_0&username=username_1&post_type=0&media_types=["png"]&media_ids=[0]"
```

**Read User Timeline**: {user_id} [start, stop]

``` zsh
curl "localhost:9000/wrk2-api/user-timeline/read" -d "user_id=USER_ID"
# e.g.
curl "localhost:9000/wrk2-api/user-timeline/read" -d "user_id=0"
curl "localhost:9000/wrk2-api/user-timeline/read" -d "user_id=1"
```

**Read Home Timeline**: {user_id} [start, stop]

``` zsh
curl "localhost:9000/wrk2-api/home-timeline/read" -d "user_id=USER_ID"
# e.g.
curl "localhost:9000/wrk2-api/home-timeline/read" -d "user_id=1"
curl "localhost:9000/wrk2-api/home-timeline/read" -d "user_id=88"
```
