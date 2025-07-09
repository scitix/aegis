#!/bin/bash

set -x

region=$1
cluster=$2
node=$3
action=$4
current_time=`date +"%Y-%m-%d %H:%M:%S"`

if [ "$action" == "breakDeadlock" ]; then
    /usr/lpp/mmfs/bin/mmdiag --deadlock -Y

    echo "begin to break gpfs deadlock"
    name=$(/usr/lpp/mmfs/bin/mmgetstate -L | grep active | awk '{print $2}')
    if [ "$name" == "" ]; then
        echo "fail to get current Node name in gpfs"
        exit 1
    fi

    echo "current Node name in gpfs: $name"
    echo yes | /usr/lpp/mmfs/bin/mmcommon breakDeadlock -N $name
    exit $?
fi

if [ "$action" == "DropCache" ]; then
    echo "before drop"
    free -g

    echo 3 > /proc/sys/vm/drop_caches

    echo "after drop"
    free -g
fi

if [ "$action" == "ConfigPeerMem" ]; then
    modprobe nvidia-peermem
    if [ $? -ne 0 ]; then
        echo "fail to modprobe nvidia-peermem"
        exit 1
    fi

    if [ ! -f /etc/modules-load.d/nvidia_peermem.conf ] && [ ! -f /etc/modules-load.d/k8s.conf ]; then
        echo nvidia_peermem >> /etc/modules-load.d/k8s.conf
    elif [ -f /etc/modules-load.d/nvidia_peermem.conf ] && [ ! -f /etc/modules-load.d/k8s.conf ]; then
        exists=$(cat /etc/modules-load.d/nvidia_peermem.conf | grep nvidia_peermem)
        if [ "$exists" == "" ]; then
            echo nvidia_peermem >> /etc/modules-load.d/nvidia_peermem.conf
        fi
    elif [ ! -f /etc/modules-load.d/nvidia_peermem.conf ] && [ -f /etc/modules-load.d/k8s.conf ]; then
        exists=$(cat /etc/modules-load.d/k8s.conf | grep nvidia_peermem)
        if [ "$exists" == "" ]; then
            echo nvidia_peermem >> /etc/modules-load.d/k8s.conf
        fi
    else
        exists=$(cat /etc/modules-load.d/nvidia_peermem.conf | grep nvidia_peermem)
        if [ "$exists" == "" ]; then
            exists=$(cat /etc/modules-load.d/k8s.conf | grep nvidia_peermem)
        fi

        if [ "$exists" == "" ]; then
            echo nvidia_peermem >> /etc/modules-load.d/nvidia_peermem.conf
        fi
    fi
fi

if [ "$action" == "RestartKubelet" ]; then
    systemctl restart containerd && systemctl restart kubelet
fi