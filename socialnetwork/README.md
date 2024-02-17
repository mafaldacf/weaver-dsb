# DeathStarBench SocialNetwork / Service Weaver

Implementation of [DeathStarBench](https://github.com/delimitrou/DeathStarBench) SocialNetwork service using Service Weaver framework, drawing inspiration from the [Blueprint](https://gitlab.mpi-sws.org/cld/blueprint)'s repository.

## Requirements

- [Golang >= 1.21](https://go.dev/doc/install)
- [Service Weaver](https://serviceweaver.dev/docs.html#installation)
- [Terraform >= v1.6.6](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli)
- [GCloud Cli](https://cloud.google.com/sdk/docs/install)

Install python packages to use the `manager` script:
```zsh
pip install -r requirements.txt
```

**OPTIONAL**: if HTTP Workload Generator (wrk2) is going to run locally
- `Lua >= 5.1.5`
- `LuaRocks >= 3.8.0` (with `lua-json`, `luasocket`, `penlight` packages)
- `OpenSSL >= 3.0.2`

```zsh
sudo apt-get install -y libssl-dev luarocks
sudo luarocks install lua-json
sudo luarocks install luasocket
sudo luarocks install penlight
```

## GCP Configuration

1. Ensure that you have a GCP project created and setup as your default project in `gcloud cli`:
``` zsh
# list all projects
gcloud projects list
# select the desired project id
gcloud config set project YOUR_PROJECT_ID
# verify it is now set as default
gcloud config get-value project
```
2. Ensure that `Compute Engine API` and `Kubernetes Engine API` and `Cloud Storage API` are enabled in GCP Dashboard
3. Create a new Cloud Storage bucket
    - Go to `GCP Dashboard -> Cloud Storage API`
    - Create a new bucket `weaver-dsb` (if you need another name, go to `socialnetwork/terraform/modules/gcp_create_instance/main.tf`, and, at the end of the resource in `metadata_startup_script`, edit the line `export GCP_BUCKET_NAME=weaver-dsb` to match your bucket name)
4. Edit the config info in `weaver-dsb/socialnetwork/gcp/config.yml` according to your GCP configuration
5. Setup a new Service Account key for authenticating (for more information: https://developers.google.com/workspace/guides/create-credentials)
    - Go to `GCP Dashboard -> IAM & Admin -> Service Accounts`
    - Select your compute engine default service account
    - Go to the keys tab and select `ADD KEY` to create a new key in JSON
    - Place your JSON file as `credentials.json` in `weaver-dsb/socialnetwork/gcp/credentials.json`
6. Setup your GCP firewall by going to `GCP Dashboard -> VPC Network -> Firewall` (or `GCP Dashboard -> Network Security -> Firewall Policies`), and add the following firewall rules:
    TODO: DEPLOY INSTANCES WITH TAGS AND APPLY FIREWALL RULES ONLY TO SPECIFIC TARGETS
   - `weaver-dsb-storage` (mongodb, rabbitmq w/ dashboard, redis and memcached):
     - targets: all instances in the network
     - source IPv4 ranges: `0.0.0.0/0`
     - TCP ports: `27017,27018,15672,15673,5672,5673,6381,6382,6383,6384,11212,11213,11214`
     - Priority: 100
   - `weaver-dsb-swarm`:
     - targets: all instances in the network
     - source IPv4 ranges: `0.0.0.0/0`
     - TCP ports: `2376,2377,7946`
     - UDP ports: `4789,7946`
     - Priority: 100

## GCP Deployment

### Deploying with GKE

Deploy datastores in GCP machines using Terraform:

``` zsh
./manager storage-deploy
./manager storage-start
```

Fetch status from docker swarm and generate app weaver config `weaver-gcp.toml` (IMPORTANT for next step!!)
``` zsh
./manager storage-info
```

Deploy application using GKE:

``` zsh
go generate
go build
weaver gke deploy weaver-gcp.toml
```

**[NOTE]** if you want to test the application locally with storages deployed in GCP use `weaver multi deploy weaver-gcp.toml` instead

Init social graph:

``` zsh
./manager init-social-graph
```

Run benchmark:

``` zsh
./manager wrk2
```

Gather metrics:
``` zsh
./manager metrics
```

Clean at the end:

``` zsh
./manager storage-clean
```

[**OPTIONAL**] to avoid daily google cloud billings, purge all GCP GKE resources:
``` zsh
weaver gke purge --force
```

## LOCAL Deployment

### Running Locally with Weaver in Multi Process

Build docker images:
``` zsh
./manager storage-build --local
```

Deploy datastores:

``` zsh
./manager storage-start --local
```

Deploy application:

``` zsh
go generate
go build
weaver multi deploy weaver.toml
```

Init social graph:

``` zsh
./manager init-social-graph --local
```

Run benchmark:

``` zsh
./manager wrk2 --local
```

Gather metrics:
``` zsh
./manager metrics --local
```

Clean datastores:

``` zsh
./manager storage-clean --local
```

### Additional

#### Manual Testing of HTTP Workload Generator

Make:

```zsh
cd wrk2
chmod +x deps/luajit/src/luajit
make
```

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

#### Manual Testing of HTTP Requests

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
OR
curl -X POST "localhost:9000/wrk2-api/user/follow" -d "user_name=USER_NAME&followee_name=FOLLOWEE_AME"

# e.g.
curl -X POST "localhost:9000/wrk2-api/user/follow" -d "user_id=1&followee_id=0"
curl -X POST "localhost:9000/wrk2-api/user/follow" -d "user_name=ana&followee_name=bob"
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
