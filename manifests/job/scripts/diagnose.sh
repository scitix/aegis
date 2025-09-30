#!/bin/bash

type=$1
count=$#
params=$@
node=$(hostname)


ExitWithTimeout () {
    sleep 5
    exit $1
}

IpmiCheck() {
    type=$1

    echo "hint: 通过 ipmi 工具获取 $type 传感器异常状态"
    echo "cmd: ipmimonitoring -Q --ignore-unrecognized-events --comma-separated-output --no-header-output --sdr-cache-recreate --output-event-bitmask --output-sensor-state -b | grep $type | grep -v Nominal"
    result=$(ipmimonitoring -Q --ignore-unrecognized-events --comma-separated-output --no-header-output --sdr-cache-recreate --output-event-bitmask --output-sensor-state -b | grep $type | grep -v Nominal)
    if [ "$result" == "" ]; then
        echo "result: ok"
    else
        echo "result:" $result
    fi
}

# node default
if [[ "$type" == "NodeNotReady" ]]; then
    ExitWithTimeout 0
fi

# baseboard
if [[ "$type" == "BaseBoardCriticalIssue" ]]; then
    IpmiCheck $2
    ExitWithTimeout $?
fi

# cpu
if [[ "$type" == "CpuUnhealthy" ]]; then
    IpmiCheck Processor
    ExitWithTimeout $?
fi

# memory
if [[ "$type" == "MemoryUnhealthy" ]]; then
    IpmiCheck Memory
    ExitWithTimeout $?
fi

# disk

# network
if [[ "$type" == "NetworkLinkDown" ]]; then
    device=$2
    echo "hint: 检查 bond0 下 slave 网卡情况"
    echo "cmd: cat /proc/net/bonding/bond0"
    result=$(nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "cat /proc/net/bonding/bond0")
    echo "result:" $result
    ExitWithTimeout 0
fi

# gpfs
if [[ "$type" == "GpfsDown" ]]; then
    ExitWithTimeout $?
fi

if [[ "$type" == "GpfsIBNotConfig" ]]; then
    echo "hint: 检查节点 gpfs 网络状态中有无 ib 设备"
    echo "cmd: /usr/lpp/mmfs/bin/mmhealth node show network | grep mlx"

    result=$(nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "/usr/lpp/mmfs/bin/mmhealth node show network | grep mlx")
    if [ "$result" == "" ]; then
        echo "result: no ib config found in gpfs"
    else
        echo "result: ok"
    fi
fi

if [[ "$type" == "IBDown" ]]; then
    if [ "$2" == "" ]; then 
        echo "hint: IB 卡数量异常"
        echo "cmd: ibstatus"
        ibstatus

        count=`nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "ibstatus" | grep "phys state" | wc -l`
        echo "result: device count $count"
        ExitWithTimeout 0
    else
        echo "hint: 检查对应 ib 卡状态是否正常"
        echo "cmd: ibstatus"

        ibstatus $2
        ib_status=`nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "ibstatus $2" | grep -v "phys state" | grep state|awk '{print $NF}'`
        if [ $ib_status != "ACTIVE" ];then
            echo "result: device $2 status $ib_status"
            ExitWithTimeout 0
        fi
    fi

    echo "result: ok"

    ExitWithTimeout $?
fi

if [[ "$type" == "IBRegisterFailed" ]]; then
    ExitWithTimeout $?
fi

# gpu
if [[ "$type" == "IBPcieDowngraded" || "$type" == "GpuPcieDowngraded" ]]; then
    echo "hint: 获取 pcie 异常的设备状态"
    echo "cmd: lspci -s$2 -vvv | grep LnkSta"
    result=""
    for((i=2;i<=count;i++)); do
        echo "lspci -s${!i} -vvv | grep LnkSta"
        trs=$(nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "lspci -s${!i} -vvv | grep LnkSta")
        if [ "$trs" != "" ]; then
            result="$result $trs"
        fi
    done

    if [ "$result" == "" ]; then
        echo "result: ok"
    else
        echo "result:" $result
    fi

    ExitWithTimeout $?
fi

if [[ "$type" == "GpuRowRemappingFailure" ]]; then
    echo "hint: 获取 gpu remapping failure 状态位"
    echo "cmd: nvidia-smi -q -d ROW_REMAPPER | grep \"Remapping Failure Occurred\""

    remapping_failures=$(nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "nvidia-smi -q -d ROW_REMAPPER" | grep "Remapping Failure Occurred" | awk '{print $NF}')

    i=0
    for remapping_failure in $remapping_failures
    do
        if [ "$remapping_failure" == "Yes" ]; then
            echo "result: gpu $i remapping failure occurred"
            ExitWithTimeout 0
        fi

        ((i=i+1))
    done

    echo "result: ok"

    ExitWithTimeout $?
fi

if [[ "$type" == "GpuAggSramUncorrectable" ]]; then
    echo "hint: 获取 gpu ecc sram uncorrectable 数值大于或等于 4 的情况"
    echo "cmd: nvidia-smi -q -d ECC | grep \"SRAM Uncorrectable\""

    ecc_sram_uncorrectable_errors=$(nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "nvidia-smi -q -d ECC" | grep "SRAM Uncorrectable" | awk '{print $NF}' | awk '!(NR%2)')
    i=0
    for ecc_sram_uncorrectable_error in $ecc_sram_uncorrectable_errors
    do
        if [ $ecc_sram_uncorrectable_error -gt 4 ]; then
            echo "result: gpu $i with $ecc_sram_uncorrectable_error(>= 4) sram uncorrectable error"
            ExitWithTimeout 0
        fi
        ((i=i+1))
    done

    echo "result: ok"
    ExitWithTimeout $?
fi

if [[ "$type" == "GpuDown" ]]; then
    expected=$2
    current=$3
    echo "hint: 查看 nvidia-smi 获取卡信息是否正常"
    echo "cmd: timeout 300 nvidia-smi"
    result=$(nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "timeout 300 nvidia-smi")
    exitcode=$?
    if [ $exitcode -eq 124 ]; then
        echo "result: nvidia-smi hang"
        ExitWithTimeout 0
    elif [ $exitcode -ne 0 ];then
        echo "result:" $result
        ExitWithTimeout 0
    else
        echo "result: ok"
    fi

    echo "hint: 查看 gpu pcie 插卡情况，是否有掉卡"
    echo "cmd: timeout 15 lspci|grep -i \"controller: nvidia\" | wc -l"
    count=$(nsenter --mount=/proc/1/ns/mnt /bin/bash -c "timeout 15 lspci|grep -i \"controller: nvidia\" | wc -l")
    if [ $count -eq $expected ]; then
        echo "result: ok (expected $expected = found $count)"
    else
        echo "result: expected gpu count $expected, found $count"
        ExitWithTimeout 0
    fi
fi

if [[ "$type" == "GpuCheckFailed" ]]; then
    echo "hint: 查看 nvidia-smi 获取卡信息是否正常"
    echo "cmd: timeout 300 nvidia-smi"
    result=$(nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "timeout 300 nvidia-smi")
    exitcode=$?
    if [ $exitcode -eq 124 ]; then
        echo "result: nvidia-smi hang"
        ExitWithTimeout 0
    elif [ $? -ne 0 ];then
        echo "result:" $result
        ExitWithTimeout 0
    else
        echo "result: ok"
    fi

    gpuCount=`nsenter --mount=/proc/1/ns/mnt -- find /dev -type c | grep -P '/nvidia[0-9]+$' | wc -l`
    echo "hint: 执行 GPU E2E 测试"
    echo "cmd: /k8s/plugins/npd/gpu_global_check -d $gpuCount"
    result=$(nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "timeout 1000 /k8s/plugins/npd/gpu_global_check -d $gpuCount")
    exitcode=$?
    if [ $exitcode -eq 124 ]; then
        echo "result: gpu e2e test hang"
        ExitWithTimeout 0
    elif [ $? -ne 0 ];then
        echo "result:" $result
        ExitWithTimeout 0
    else
        echo "result: ok"
    fi
    
    ExitWithTimeout $?
fi

if [[ "$type" =~ ^Xid ]]; then
    device=$2
    code=$3
    echo "hint: 查看对应的 GPU XID 异常信息"
    echo "cmd: dmesg -T | grep Xid | grep $code"
    result=$(nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "dmesg -T | grep Xid | grep $code")
    if [ "$result" == "" ]; then
        echo "result: ok"
    else
        echo "result:" $result
    fi
    ExitWithTimeout 0
fi

if [[ "$type" == "GpuRegisterFailed" ]]; then
    echo "hint: 查看 nvidia 驱动 Pod 异常状态"

    echo "hint: 查看 nvidia 驱动 Pod 错误日志"
fi

if [[ "$type" == "NvidiaFabricManagerNotActive" ]]; then
    echo "hint: 查看 nvidia-fabricmanager 服务状态"
    echo "cmd: systemctl status nvidia-fabricmanager"
    result=$(nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "systemctl status nvidia-fabricmanager")
    echo "result:" $result
fi

if [[ "$type" == "GpuPersistenceModeNotEnabled" ]]; then
    echo "hint: 查看 nvidia-fabricmanager 服务状态"
    echo "cmd: nvidia-smi -q | \"Persistence Mode\""
    result=$(nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "nvidia-smi -q" | 'Persistence Mode')
    echo "result:" $result
fi

if [[ "$type" == "GpuNvlinkInactive" ]]; then
    echo "hint: 查看 nvidia nvlink 状态"
    echo "cmd: nvidia-smi nvlink -s"
    result=$(nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "nvidia-smi nvlink -s")
    echo "result:" $result
fi

if [[ "$type" == "GpuNvlinkError" ]]; then
    echo "hint: 执行 sichek nccl 测试"
    echo "cmd: sichek nccltest"
    result=$(nsenter --mount=/proc/1/ns/mnt -- /bin/bash -c "sichek nccltest")
    echo "result:" $result
fi