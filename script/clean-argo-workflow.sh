#!/bin/bash

echo "start to clean completed argo workflows."

kubectl get workflow -A --sort-by=.status.finishedAt > workflow.info
count=$(cat workflow.info | wc -l)
to_delete_count=`expr $count / 5`

echo "cluster workflow count: $count, plan to clean $to_delete_count (20% of total count)."

cat workflow.info | grep Succeeded > workflow.completed
cat workflow.info | grep Failed >> workflow.completed
completed_count=$(cat workflow.completed | wc -l)
echo "cluster completed worklflow count: $completed_count"

head -n $to_delete_count workflow.completed > workflow.to_delete
cat workflow.to_delete | awk '{print "kubectl delete workflow " $2 " -n " $1}' > clean-workflows.sh

bash clean-workflows.sh

echo "end!"