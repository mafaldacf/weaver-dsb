# DeathStarBench SocialNetwork @ Service Weaver

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

## Configuration

### GCP Configuration

1. Ensure that you have a GCP project created and setup as your default project in `gcloud cli`:
``` zsh
# list all projects
gcloud projects list
# select the desired project id
gcloud config set project YOUR_PROJECT_ID
# verify it is now set as default
gcloud config get-value project
```
1. Ensure that [Compute Engine API](https://console.cloud.google.com/marketplace/product/google/compute.googleapis.com) and [Kubernetes Engine API](https://console.cloud.google.com/marketplace/product/google/container.googleapis.com), [Cloud Storage API](https://console.cloud.google.com/marketplace/product/google/storage.googleapis.com), and [Artifact Registry API](https://console.cloud.google.com/marketplace/product/google/artifactregistry.googleapis.com) are enabled in GCP
2. Go to `weaver-dsb/socialnetwork/gcp/config.yml` and place
   1. your GCP `project_id`
   2. any desired `username` to access GCP instances
   3. any desired `bucket_name` (e.g. `weaver-dsb` with some suffix to ensure it's unique) for creating a new bucket
3. Create a new Cloud Storage bucket and create new Firewall Rules by passing your bucket name
``` zsh
./manager.py configure --gcp --bucket YOUR_BUCKET_NAME
```
5. Setup a new Service Account key for authenticating (for more information: https://developers.google.com/workspace/guides/create-credentials)
    - Go to [IAM & Admin -> Service Accounts](https://console.cloud.google.com/iam-admin/serviceaccounts) of your project
    - Select your compute engine default service account
    - Go to the keys tab and select `ADD KEY` to create a new key in JSON
    - Place your JSON file as `credentials.json` in `weaver-dsb/socialnetwork/gcp/credentials.json`

### Terraform Configuration

In the terraform folder, go to `scripts/deps-storage` and edit the `GCP_BUCKET_NAME` environment variable

### Workload Configuration

Generate workload binary:

```zsh
cd wrk2
make
```

## Application Deployment

### Local Deployment

#### Weaver Multi Process

Build docker images and deploy datastores (mongodb, redis, rabbitmq):
``` zsh
./manager.py storage-build --local
./manager.py storage-run --local
```

Deploy and run application:

``` zsh
go generate
go build
weaver multi deploy weaver-local.toml
```

[**OPTIONAL**] Init social graph (not necessary if workload only runs sequences of `ComposePost`, which the default for now):

``` zsh
./manager.py init-social-graph --local
```

Run benchmark (automatically gathers metrics in new file in `evaluation` directory):

``` zsh
# default params: 2 threads, 2 clients, 30 duration (in seconds), 50 rate
./manager.py wrk2 --local

# if you want to specify some params
./manager.py wrk2 --local -t THREADS -c CLIENTS -d DURATION -r RATE
# values used antipode evaluation:
# threads, clients, rate
#   2        4        50
#   2        4        100
#   2        4        125
#   2        4        150
#   2        4        160
```

If you want to just observe metrics:
``` zsh
./manager.py metrics --local
```

Clean datastores:
``` zsh
./manager.py storage-clean --local
```

### GCP Deployment

#### Deploying in GCP machines for app + datastores

Build, deploy, and run your application
``` zsh
./manager.py build --gcp
./manager.py deploy --gcp
./manager.py run --gcp
```

If you want to display some info
``` zsh
./manager.py info --gcp
```

Run workload for benchmarking application and print metrics
``` zsh
# default params: 2 threads, 2 clients, 5 duration (in seconds), 5 rate
# values used antipode evaluation:  threads; 4 clients; 300 duration; 50, 100, 125, 150, and 160 rates
./manager.py wrk2 --gcp [-t THREADS] [-c CLIENTS] [-d DURATION] [-r RATE]
./manager.py metrics --gcp
```

If you want to restart your metrics or storages, do:
``` zsh
./manager.py restart --gcp
```

Otherwise, to terminate storages and application at the end, do:

``` zsh
./manager.py clean --gcp
```

## Additional Info

### Manual Testing of HTTP Workload Generator

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

### Manual Testing of HTTP Requests

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
