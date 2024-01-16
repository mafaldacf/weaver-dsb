Single Process

``` zsh
    go generate
    SERVICEWEAVER_CONFIG=weaver.toml go run .
```

Multi Process

``` zsh
    go build
    weaver multi deploy weaver.toml
```

Setup MongoDB and RabbitMQ locally

``` zsh
    docker run --name mongodb-primary -p 27017:27017 mongo:4.2
    docker run --name rabbitmq -p 15672:15672 -p 5672:5672 rabbitmq:3.8-management

    docker-compose up -d
    docker exec -it b1bee6e9a879 mongo --eval "rs.status()"
```

Setup Docker Swarm

``` zsh
    # host
    # TODO: create GCP machine instances

    # manager
    docker swarm init --advertise-addr 34.159.133.11:2377

    # host -> manager + all nodes
    scp docker-compose.yml mafaldacf@34.159.133.11:
    scp -r config mafaldacf@34.159.133.11:
    scp -r docker mafaldacf@34.159.133.11:


    # manager + nodes
    docker build -t mongodb-delayed:4.4.6 ~/docker/mongodb-delayed/.
    docker build -t mongodb-setup:4.4.6 ~/docker/mongodb-setup/post-storage/.
    docker build -t rabbitmq-setup:3.8 ~/docker/rabbitmq-setup/write-home-timeline/.

    # manager
    docker swarm init --advertise-addr 34.159.6.214:2377
    docker network create --attachable -d overlay deathstarbench_network --advertise-addr 34.159.6.214:2377
    docker stack deploy --with-registry-auth --compose-file docker-compose.yml socialnetwork
    #docker stack ps socialnetwork
    #docker service ls

    # nodes
    # must specify `--advertise-addr <node_public_ip>`
    # otherwise, the dns resolution does not work properly
    # https://stackoverflow.com/questions/53007038/docker-swarm-container-cannot-resolve-address-of-container-in-another-node
    # https://github.com/moby/swarmkit/issues/1429#issuecomment-329325410
    docker swarm join --token SWMTKN-1-2emsmnoczckchd12i4krxpprevmxc8b1sw8vsdgh72ie1huuqp-42os0rp2zng4ver8duakrpf1m 34.159.133.11:2377 --advertise-addr 34.159.6.214
    docker swarm join --token SWMTKN-1-2emsmnoczckchd12i4krxpprevmxc8b1sw8vsdgh72ie1huuqp-42os0rp2zng4ver8duakrpf1m 34.159.133.11:2377 --advertise-addr 35.238.156.70
    
    # nodes
    docker swarm leave

    # manager
    docker stack rm socialnetwork
```
