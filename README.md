# Kubestr

Kubestr is a collection of tools that discover and evaluate the storage options present in a kubernetes cluster. 
To run the tool -  
- Ensure that the kubernetes context is set and the cluster is accessible through your terminal. (Does [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) work?)

- Download the latest release for [mac](https://github.com/kastenhq/kubestr/releases/download/0.3.2/kubestr-0.3.2-darwin-amd64.tar.gz), [windows](https://github.com/kastenhq/kubestr/releases/download/0.3.2/kubestr-0.3.2-windows-amd64.zip) or [linux](https://github.com/kastenhq/kubestr/releases/download/0.3.2/kubestr-0.3.2-linux-amd64.tar.gz) and run `./kubestr`

To run the FIO test - 
- `./kubestr fio -s <storage class>`

To check a CSI drivers snapshot and restore capabilities - 
- `./kubestr csicheck -s <storage class> -v <volume snapshot class>`
