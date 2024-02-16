# DeathStarBench SocialNetwork / Service Weaver

Implementation of [DeathStarBench](https://github.com/delimitrou/DeathStarBench) SocialNetwork service using Service Weaver framework, drawing inspiration from the [Blueprint](https://gitlab.mpi-sws.org/cld/blueprint)'s repository.

## Requirements

- [Golang >= 1.21](https://go.dev/doc/install)
- [Terraform >= v1.6.6](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli)
- [Service Weaver](https://serviceweaver.dev/docs.html#installation)

If HTTP Workload Generator (wrk2) is used locally
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

Place your `GCP credentials JSON file` (e.g. using Service Account as authentication method) in `weaver-dsb/socialnetwork/gcp`

For more information: https://developers.google.com/workspace/guides/create-credentials

## Deployment

### Running Locally with Weaver in Single Process

Deploy datastores:

``` zsh
docker-compose up -d
```

Deploy application:

``` zsh
go generate
SERVICEWEAVER_CONFIG=weaver.toml go run .
```

### Running Locally with Weaver in Multi Process

Deploy datastores:

``` zsh
docker-compose up -d
```

Deploy application:

``` zsh
go generate
go build
weaver multi deploy weaver.toml
```

### Running in GKE

Deploy datastores in GCP machines using Terraform:

``` zsh
./manager deploy
```

Deploy application using GKE:

``` zsh
go build
weaver gke deploy config/weaver/weaver-eu.toml
weaver gke deploy config/weaver/weaver-us.toml
```

Run benchmark in GKE:

``` zsh
./manager run
./manager info
```

Clean at the end:

``` zsh
./manager clean
```

## Initializing Social Graph

```zsh
source config.sh
cd scripts
pip install -r requirements.txt
python3 init_social_graph.py
```

## Running HTTP Workload Generator

### Make

```zsh
cd wrk2
chmod +x deps/luajit/src/luajit
make
```

### Compose Posts

```zsh
cd wrk2
./wrk -D exp -t <num-threads> -c <num-conns> -d <duration> -L -s ./scripts/social-network/compose-post.lua http://localhost:9000/wrk2-api/post/compose -R <reqs-per-sec>

# e.g.
./wrk -D exp -t 1 -c 1 -d 1 -L -s ./scripts/social-network/compose-post.lua http://localhost:9000/wrk2-api/post/compose -R 1
```

### Read Home Timelines

```zsh
cd wrk2
./wrk -D exp -t <num-threads> -c <num-conns> -d <duration> -L -s ./scripts/social-network/read-home-timeline.lua http://localhost:9000/wrk2-api/home-timeline/read -R <reqs-per-sec>
```

### Read User Timelines

```zsh
cd wrk2
./wrk -D exp -t <num-threads> -c <num-conns> -d <duration> -L -s ./scripts/social-network/read-user-timeline.lua http://localhost:9000/wrk2-api/user-timeline/read -R <reqs-per-sec>
```

### Sequence Compose Post >> Read Home Timelines

```zsh
cd wrk2
./wrk -D exp -t <num-threads> -c <num-conns> -d <duration> -L -s ./scripts/social-network/sequence-compose-post-read-home-timeline.lua http://localhost:9000/wrk2-api/home-timeline/read -R <reqs-per-sec>

# e.g.
./wrk -D exp -t 1 -c 1 -d 1 -L -s ./scripts/social-network/sequence-compose-post-read-home-timeline.lua http://localhost:9000/wrk2-api/home-timeline/read -R 1
```

## Running Lua Scripts Manually

```zsh
lua ./scripts/social-network/sequence-compose-post-read-home-timeline.lua
```

## Manual HTTP Requests

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
