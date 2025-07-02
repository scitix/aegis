#!/bin/bash
set -e
set -x

ACTION=$1
shift

case "$ACTION" in
  diagnose)
    /opt/aegis/diagnose.sh "$@"
    ;;
  healthcheck)
    cp /opt/aegis/healthcheck.sh /var/selfhealing/
    cp /opt/aegis/check_gpu_status.py /var/selfhealing/
    nsenter -m/proc/1/ns/mnt -- chmod +x /var/selfhealing/healthcheck.sh
    nsenter -m/proc/1/ns/mnt -- mkdir -p /scratch/AIInfra/selfhealing/healthcheck
    nsenter -m/proc/1/ns/mnt -- /bin/bash -c "cd /var/selfhealing/ && ./healthcheck.sh $@"
    ;;
  remedy)
    cp /opt/aegis/remedy.sh /tmp/
    chmod +x /tmp/remedy.sh
    nsenter -m/proc/1/ns/mnt -- /bin/bash -c "cd /tmp && ./remedy.sh $@"
    ;;
  repair)
    cp /opt/aegis/repair.sh /tmp/
    chmod +x /tmp/repair.sh
    nsenter -m/proc/1/ns/mnt -- /bin/bash -c "cd /tmp && ./repair.sh $@"
    ;;
  reboot)
    cp /opt/aegis/restart_node.sh /var/selfhealing/
    nsenter -m/proc/1/ns/mnt -- chmod +x /var/selfhealing/restart_node.sh
    nsenter -m/proc/1/ns/mnt -- /var/selfhealing/restart_node.sh
    ;;
  shutdown)
    cp /opt/aegis/shutdown_node.sh /var/selfhealing/
    nsenter -m/proc/1/ns/mnt -- chmod +x /var/selfhealing/shutdown_node.sh
    nsenter -m/proc/1/ns/mnt -- /var/selfhealing/shutdown_node.sh
    ;;
  *)
    echo "Unknown action: $ACTION"
    exit 1
    ;;
esac
