#!/bin/bash

set -e
set -x

reboot_lock_file=reboot.lock

if [ -f "$reboot_lock_file" ]; then
  echo "find reboot lock file exist, so machine just be rebooted, exit 0"
  rm -f $reboot_lock_file

  os=$(awk -F= '/^NAME/{print $2}' /etc/os-release)
  if [ "$os" == "Ubuntu" ]; then
    bash /root/mount.sh
  fi

  exit 0
else
  [ ! -f "$reboot_lock_file" ] && touch $reboot_lock_file

  systemctl enable kubelet
  if [ "$(systemctl is-active containerd)" = "active" ]; then
      systemctl enable containerd
  fi

  echo "begin to shutdown gpfs"
  /usr/lpp/mmfs/bin/mmshutdown

  echo "begin reboot machine after sleep 3 second"
  
  sleep 3 && reboot
fi