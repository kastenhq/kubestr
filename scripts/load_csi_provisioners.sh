#!/usr/bin/env bash

curl https://raw.githubusercontent.com/kubernetes-csi/docs/master/book/src/drivers.md -o ../extra/csi-drivers

while read p; do
  if [[ $p == [* ]];
  then
    IFS='|'
    read -a fields <<< "$p"
    jq -n '{ "url": ${fields[0]}, "name": ${fields[1]} }'
  fi
done <../extra/csi-drivers