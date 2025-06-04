#!/bin/bash

# clean iptables coredump file
docker exec -t kube-proxy /bin/bash -c "rm -f core*"

# clean big log file
logfiles=$(find /var/lib/docker/containers/ | grep json.log)
while read -r line
do
    size=$(ls -lh $line | awk '{print $5}')
    if [[ $size == *"G"* ]]; then
        truncate -s 0 $line
        echo "truncate file $line, size: $size"
    fi
done <<< "$logfiles"