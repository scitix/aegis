#!/bin/bash

set -x

region=$1
cluster=$2
node=$3
current_time=`date +"%Y-%m-%d %H:%M:%S"`

# cpu performance
for CPUREQ in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor;do echo -n performance> $CPUREQ; done

ExitWithTimeout () {
    echo "sleep 5 and exit with $1"
    sleep 5
    exit $1
}

timeout 15 lspci|grep -i nvidia > /dev/null 2>&1
if [ $? -eq 0 ];then
    echo "===================`date +"%Y-%m-%d %H:%M:%S"` checking this is GPU node..."
    timeout 300 nvidia-smi > /dev/null 2>&1
    if [ $? -eq 0 ];then
        echo "===================`date +"%Y-%m-%d %H:%M:%S"` checking nvidia-smi success..."
        sudo nvidia-smi -pm 1 > /dev/null 2>&1
    else
        echo "===================`date +"%Y-%m-%d %H:%M:%S"` checking nvidia-smi failed..."
        ExitWithTimeout 2
    fi
    timeout 300 nvidia-smi -L > /dev/null 2>&1
    if [ $? -eq 0 ];then
        echo "===================`date +"%Y-%m-%d %H:%M:%S"` checking nvidia-smi -L success..."
    else
        echo "===================`date +"%Y-%m-%d %H:%M:%S"` checking nvidia-smi -L failed..."
        ExitWithTimeout 2
    fi

    remapping_failures=$(nvidia-smi -q -d ROW_REMAPPER | grep "Remapping Failure Occurred" | awk '{print $NF}')
    for remapping_failure in $remapping_failures
    do
        if [ "$remapping_failure" == "Yes" ]; then
            echo "===================`date +"%Y-%m-%d %H:%M:%S"` checking nvidia-smi gpu Remapping Failure Occurred: $remapping_failure"
            ExitWithTimeout 21
        fi
    done

    ecc_sram_uncorrectable_errors=$(nvidia-smi -q -d ECC | grep "SRAM Uncorrectable" | awk '{print $NF}' | awk '!(NR%2)')
    for ecc_sram_uncorrectable_error in $ecc_sram_uncorrectable_errors
    do
        if [ $ecc_sram_uncorrectable_error -gt 4 ]; then
            echo "===================`date +"%Y-%m-%d %H:%M:%S"` checking nvidia-smi gpu with too many sram uncorrectable error: $ecc_sram_uncorrectable_error"
            ExitWithTimeout 22
        fi
    done

    gpuCount=`find /dev -type c | grep -P '/nvidia[0-9]+$' | wc -l`
    /k8s/plugins/npd/gpu_global_check -d $gpuCount
    if [ $? -eq 0 ]; then
        echo "===================`date +"%Y-%m-%d %H:%M:%S"` checking gpu status success..."
    else
        echo "===================`date +"%Y-%m-%d %H:%M:%S"` checking gpu status failed..."
        ExitWithTimeout 23
    fi
else
    echo "===================`date +"%Y-%m-%d %H:%M:%S"` this is not GPU node..."
fi

ExitWithTimeout 0