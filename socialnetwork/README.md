# Deployment

## Local + Single Process

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

## Local + Multi Process

Deploy datastores:
``` zsh
    docker-compose up -d
```

Deploy application:
``` zsh
    go build
    weaver multi deploy weaver.toml
```

Sending HTTP requests to available endpoints
``` zsh
    # register user
    curl -X POST "localhost:9000/wrk2-api/user/register" -d "username=user_0&user_id=0&first_name=m&last_name=f&password=123"
    # follow user
    curl -X POST "localhost:9000/wrk2-api/user/follow"
    # unfollow user
    curl -X POST "localhost:9000/wrk2-api/user/unfollow"
    # compose post
    curl -X POST "localhost:9000/wrk2-api/post/compose"
    # read home timeline
    curl "localhost:9000/wrk2-api/home/timeline"
    # read user timeline
    curl "localhost:9000/wrk2-api/user/timeline"
```

## GKE + datastores

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
