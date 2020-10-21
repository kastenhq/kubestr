#!/usr/bin/env bash

# COLOR CONSTANTS
GREEN='\033[0;32m'
LIGHT_BLUE='\033[1;34m'
RED='\033[0;31m'
NC='\033[0m'

print_heading() {
    printf "${LIGHT_BLUE}$1${NC}\n"
}

print_error(){
    printf "${RED}$1${NC}\n"
}

print_success(){
    printf "${GREEN}$1${NC}\n"
}

readonly -a REQUIRED_TOOLS=(
    kubectl
)

DEFAULT_IMAGE_TAG="latest"
DEFAULT_PV_SIZE="100"
DEFAULT_JOB_NAME="kubestr-fio"

helpFunction()
{
    echo ""
    echo "This scripts runs Kubestr FIO tests as a Job in a cluster"
    echo "Usage: $0 -i image -n namespace -s storageclass -z sizeInGiB -f fioConfigFile"
    echo -e "\t-i The Kubestr FIO image"
    echo -e "\t-n The kubernetes namespace where the job will run"
    echo -e "\t-s The Storage Class to use when running FIO tests. (Required)"
    echo -e "\t-z The Size of the Persistent Volume in GiB. Default: 1000."
    echo -e "\t-f An FIO config file"
    exit 1 # Exit script after printing help
}

while getopts "i:n:s:z:f:" opt
do
    case "$opt" in
        i ) image="$OPTARG" ;;
        n ) namespace="$OPTARG" ;;
        s ) storageclass="$OPTARG" ;;
        z ) size="$OPTARG" ;;
        f ) fioconfig="$OPTARG" ;;
        ? ) helpFunction ;; # Print helpFunction in case parameter is non-existent
    esac
done

if [ -z "$namespace" ]
then
    echo "Namespace option not provided, using default namespace";
    namespace="default"
fi

if [ -z "$storageclass" ]
then
    print_error "The Storage Class option (-s) is required."
    helpFunction
    exit 1
fi

if [ -z "$size" ]
then
    echo "Size option not provided, using default size ${DEFAULT_PV_SIZE}Gi"
    size=${DEFAULT_PV_SIZE}
fi

check_tools() {
  print_heading "Checking for tools"
  for tool in "${REQUIRED_TOOLS[@]}"
  do
    if ! command -v "${tool}" > /dev/null 2>&1
    then
      print_error " --> Unable to find ${tool}"
      failed=1
    else
      print_success " --> Found ${tool}"
    fi
  done
}

check_image() {
  print_heading "Kubestr image"
  if [ -z "$image" ]
  then
      # need to change this to public dockerhub
      image=ghcr.io/kastenhq/kubestr:${DEFAULT_IMAGE_TAG}
  fi
  print_success " --> ${image}"
}


failed=0
check_tools && check_image
if [[ ${failed} != 0 ]]; then
    print_error "Pre-checks failed"
    exit 1
fi

print_heading "Running Kubestr FIO Job in ${namespace} namspace"

cat > kuberstr-fio.yaml << EOF
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: ${DEFAULT_JOB_NAME}-pv-claim
  namespace: ${namespace}
spec:
  storageClassName: ${storageclass}
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: ${size}Gi
---
apiVersion: batch/v1
kind: Job
metadata:
  name: ${DEFAULT_JOB_NAME}
  namespace: ${namespace}
spec:
  template:
    spec:
      containers:
      - name: ${DEFAULT_JOB_NAME}
        image: ${image}
        command: [ "/fio.sh" ]
        args: [ "fio" ]
        imagePullPolicy: Always
        env:
          - name: DBENCH_MOUNTPOINT
            value: /data
        volumeMounts:
        - name: ${DEFAULT_JOB_NAME}-pv
          mountPath: /data
      restartPolicy: Never
      volumes:
      - name: ${DEFAULT_JOB_NAME}-pv
        persistentVolumeClaim:
          claimName: ${DEFAULT_JOB_NAME}-pv-claim
  backoffLimit: 4
EOF

kubectl apply -f kuberstr-fio.yaml

trap "kubectl delete -f kuberstr-fio.yaml" EXIT

while [[ $(kubectl -n ${namespace} get pods --selector=job-name=${DEFAULT_JOB_NAME} -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" && $(kubectl -n ${namespace} get pods --selector=job-name=${DEFAULT_JOB_NAME} -o 'jsonpath={..phase}') != "Succeeded" ]];
do echo "Waiting for pod $(kubectl -n ${namespace} get pods --selector=job-name=${DEFAULT_JOB_NAME} --output=jsonpath='{.items[*].metadata.name}') to be ready - $(kubectl -n ${namespace} get pods --selector=job-name=${DEFAULT_JOB_NAME} -o 'jsonpath={..status.containerStatuses[0].state.waiting.reason}')" && sleep 1;
done
echo "Pod Ready!"
echo ""
pod=$(kubectl -n ${namespace} get pods --selector=job-name=${DEFAULT_JOB_NAME} --output=jsonpath='{.items[*].metadata.name}')
kubectl logs -n ${namespace} ${pod} -f
echo ""
