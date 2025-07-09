#!/bin/bash

component=$1

if [ "$component" == "NETWORK" ]
then
    /usr/lpp/mmfs/bin/mmhealth node show network
elif [ "$component" == "NODE" ]
then
    /usr/lpp/mmfs/bin/mmhealth node show --unhealthy
    /usr/lpp/mmfs/bin/mmhealth node eventlog --day
else
    echo "unsupported diagnosis component $component"
    exit 1
fi