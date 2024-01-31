# Requirements

- [Golang >= 1.21](https://go.dev/doc/install)
- [Terraform >= v1.6.6](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli)
- [Service Weaver](https://serviceweaver.dev/docs.html#installation)

# Configuration

Place your GCP credentials JSON file (e.g. using Service Account as authentication method) in `weaver-dsb/socialnetwork/gcp`

For more: https://developers.google.com/workspace/guides/create-credentials

# Deployment

## Running Locally with Weaver in Single Process

Deploy datastores:
``` zsh
    docker-compose up -d
```

Deploy application:
``` zsh
    docker-compose up -d
    go generate
    SERVICEWEAVER_CONFIG=weaver.toml go run .
```

## Running Locally with Weaver in Multi Process

Deploy datastores:
``` zsh
    docker-compose up -d
```

Deploy application:
``` zsh
    go build
    weaver multi deploy weaver.toml
```

## Running in GKE

Deploy datastores in GCP machines:
``` zsh
    ./manager terraform-deploy
    ./manager run
```

Deploy application using GKE:
``` zsh
    go build
    weaver gke deploy config/weaver/weaver-eu.toml
    weaver gke deploy config/weaver/weaver-us.toml
```

## Sending Manual HTTP Requests

**Register User**: {username, first_name, last_name, password} [user_id]
``` zsh
    curl -X POST "localhost:9000/wrk2-api/user/register" -d "username=user_0&user_id=0&first_name=m&last_name=f&password=123"
```
**Follow User**: [{user_id, followee_id}, {username, followee_name}]
``` zsh
    curl -X POST "localhost:9000/wrk2-api/user/follow" -d "user_id=1&followee_id=0"
```
**Unfollow User**: [{user_id, followee_id}, {username, followee_name}]
``` zsh
    curl -X POST "localhost:9000/wrk2-api/user/unfollow" -d "user_id=0&followee_id=1"
```
**Compose Post**: {user_id, text, username, post_type} [media_types, media_ids]
``` zsh
    curl -X POST "localhost:9000/wrk2-api/post/compose" -d "user_id=0&text=helloworld&username=user_0&post_type=0"
```
**Read Home Timeline**: {user_id} [start, stop]
``` zsh
    curl "localhost:9000/wrk2-api/home-timeline/read" -d "user_id=1"
```
**Read User Timeline**: {user_id} [start, stop]
``` zsh
    curl "localhost:9000/wrk2-api/user-timeline/read" -d "user_id=0"
```

e.g.
``` zsh
    curl -X POST "localhost:9000/wrk2-api/user/register" -d "username=user_0&user_id=0&first_name=ana&last_name=ana2&password=123"
    curl -X POST "localhost:9000/wrk2-api/user/register" -d "username=user_1&user_id=1&first_name=bob&last_name=bob2&password=123"
    curl -X POST "localhost:9000/wrk2-api/user/follow" -d "user_id=1&followee_id=0"
    curl -X POST "localhost:9000/wrk2-api/post/compose" -d "user_id=0&text=helloworld&username=user_0&post_type=0"
    curl "localhost:9000/wrk2-api/user-timeline/read" -d "user_id=0"
```
