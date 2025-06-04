#!/bin/bash

export LANG=C.UTF-8

node=$1
address=$2
alert=$3
type=$4
object="Node/$node"
group=$5

echo $@

notify() 
{
    message=$1
    content=$(printf '{"title": "Aegis Workflow - Make Descision", "alert": "%s", "type": "%s", "object": "%s", "message": "%s", "group": "%s"}' "$alert" "$type" "$object" "$message" "$group")
    curl -X POST $address -H "Content-Type: application/json" -d "$content"
}

if [ -z "$node" ]; then
	echo "empty node"
	help
	exit 1
fi

# 校验节点
kubectl get node $node
if [ $? -ne 0 ]; then
    echo "node $node not found in cluster"
    help
    exit 1
fi

# check ubiloader pod
ubiloader=$(kubectl -n hi-sys get pods -owide | grep $node | grep "ubiloader-data" | awk '{print $1}')
if [ "$ubiloader" != "" ]; then
    echo "find ubiloader pod $ubiloader in node $node"
    output=$(kubectl -n hi-sys exec -t $ubiloader -- du -shb /tmp)
    echo "du -shb /tmp: $output"
    if [ $? -ne 0 ]; then
        exit 1
    fi
    size=$(echo $output | awk '{print $1}')

    if (( $size > 10737418240 )); then
        size=`expr $size / 1024 / 1024 / 1024`
        kubectl -n hi-sys exec -t $ubiloader -- /bin/bash -c 'cd /tmp && find -name "*log" -delete'
        if [ $? -ne 0 ]; then
            message="succeed clean $size GB log file in ubiloader pod $ubiloader in node $node"
        else
            message="failed clean $size GB log file in ubiloader pod $ubiloader in node $node"
        fi

        echo $message
        notify $message
    fi
fi