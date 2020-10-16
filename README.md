# Kubestr

Kubestr is a tool that qualifies the storage options present in a cluster.  
In upcoming releases we plan to suport running an FIO test on the storage as well as testing the snapshotting capabilities of a storage provisioner.

To run the tool -  
`curl https://kastenhq.github.io/kubestr/scripts/run_kubestr.sh | bash`

## Developers

### Building and testing the tool locally -  
(You must have [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) installed and kubernetes context must be set)

Build  
`go build`

Run  
`./kubestr`

### To create your own image - 
 
Build a docker image -  
`docker build . --file Dockerfile --tag kubestr`  

Tag it with your personal repo -  
`docker tag kubestr:latest <yourRepo:version>`  

Push it! -  
`docker push <yourRepo:version>`

You can now run your image in a pod by executing the run_kubestr script -  
`curl https://kastenhq.github.io/kubestr/scripts/run_kubestr.sh/run_kubestr.sh | bash /dev/stdin -i  <yourRepo:version>`


