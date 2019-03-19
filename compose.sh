#!/bin/sh

NREPS="$1"
if [ -z "$NREPS" ]; then
    echo "Usage: ./compose.sh <nreps> <args>..."
    exit 1
fi
if [ "$NREPS" -gt "10" ]; then
    echo "Maximum 10 replicas for docker-compose"
    exit 2
fi
shift

VALID=
if [ -f "$0" ] && [ -f "./docker-compose.yml" ] && [ "$0" -ot "./docker-compose.yml" ]; then
    OLD=$(docker-compose config --services | wc -l)
    if [ "$NREPS" = "$OLD" ]; then
        echo "Config file good"
        VALID=1
    fi
fi

if [ -z "$VALID" ]; then
    if [ -f "./docker-compose.yml" ]; then
        docker-compose down
    fi

    cat <<EOF >docker-compose.yml
# DO NOT EDIT: Automatically generated by compose.sh
version: "2.3"
services:
EOF

    I=0
    while [ "$I" -lt "$NREPS" ]; do
        cat <<EOF >>docker-compose.yml

  epaxos-server-$I:
    image: b1f6c1c4/epaxos
    container_name: epaxos-server-$I
    restart: always
    environment:
      EPAXOS_DEBUG: "TRUE"
      EPAXOS_LISTEN: "0.0.0.0:23330"
      EPAXOS_NREPLICAS: "$NREPS"
      EPAXOS_REPLICA_ID: "$I"
      EPAXOS_SERVERS_FMT_BIAS: "0"
      EPAXOS_SERVERS_FMT: "epaxos-server-%d:23330"
      EPAXOS_DATA_PREFIX: "/data/epaxos/data-"
    ports:
      - "$((23330+$I)):23330/udp"
    volumes:
      - ./data/data-$I:/data/epaxos
    networks:
      - epaxos
EOF
    I=$(($I+1))
    done

    cat <<EOF >>docker-compose.yml

networks:
  epaxos:
    driver: bridge
EOF

    echo "Config file regenerated"
fi

if [ ! "$#" = 0 ]; then
    docker-compose "$@"
fi
