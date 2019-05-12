#!/bin/sh

set -euxo pipefail

pv --version >&2

NS=${1:-5}    # Number of servers
NR=${2:-1024} # Number of records to test
NP=${3:-128}  # Client window
RK=${4:-5}    # Key collsion is 5%
D0=${5:-100}  # Minimum delay
DD=${6:-50}   # Incremental delay
JT=${7:-25}   # Std.Var. is 10% of mean

NK=$(($NR * 100 / $RK))

make -j2 debug >&2
make pumba-down >&2
docker-compose down

sudo find ./data -type f -delete

./compose.sh "$NS" --prod up -d

for I in $(seq 0 $(($NS-1))); do
    docker run -d --name "epaxos-pumba-delay-$I" \
        --volume /var/run/docker.sock:/var/run/docker.sock \
        gaiaadm/pumba --log-level info \
        netem --duration 1000000h \
        delay --time "$(($D0 + $I * $DD))" \
        --jitter "$((($D0 + $I * $DD) * $JT / 100))" \
        "epaxos-server-$I" >&2
done

for I in $(seq 0 $(($NS-1))); do
    ./bin/client \
        -n "$NS" -t 30.0 --verbose \
        batch-put \
        -N "$NR" \
        --pipeline "$NP" \
        --latency --random-key \
        "$I" "$NK" \
        2>"data/client-$I.log" \
        | pv -s "$NR" -l \
        | sed "s/^/$NS,$NR,$NP,$RK,$D0,$DD,$JT,$I,/"
done

make pumba-down >&2
./compose.sh "$NS" --prod down
