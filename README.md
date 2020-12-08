# Kubestr

Kubestr is a collection of tools discover and validate the storage options present in a kubernetes cluster.  

To run the tool -  
Download the latest release for [mac](https://github.com/kastenhq/kubestr/releases/download/0.3.2/kubestr-0.3.2-darwin-amd64.tar.gz), [windows](https://github.com/kastenhq/kubestr/releases/download/0.3.2/kubestr-0.3.2-windows-amd64.zip) or [linux](https://github.com/kastenhq/kubestr/releases/download/0.3.2/kubestr-0.3.2-linux-amd64.tar.gz and run `./kubestr`

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
`curl https://kastenhq.github.io/kubestr/scripts/run_kubestr.sh | bash /dev/stdin -i  <yourRepo:version>`


