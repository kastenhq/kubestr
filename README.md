# Kubestr

Discover your kubernetes storage options.

Kubestr is a collection of tools to discover, validate and evaluate your kubernetes storage options.

As adoption of kubernetes grows so have the persistent storage offerings that are available to users. The introduction of [CSI](https://kubernetes.io/blog/2019/01/15/container-storage-interface-ga/)(Container Storage Interface) has enabled storage providers to develop drivers with ease. In fact there are around a 100 different CSI drivers available today. With all these options it can become a bit daunting to choose the right storage.

Kubestr can assist in the following ways-
- Identify the various storage options present in a cluster.
- Validate if the storage options are configured correctly.
- Evaluate the storage using common benchmarking tools like FIO.

## Using Kubestr
### To install the tool -  
- Ensure that the kubernetes context is set and the cluster is accessible through your terminal. (Does [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) work?)
- Download the latest release for [mac](https://github.com/kastenhq/kubestr/releases/download/0.3.2/kubestr-0.3.2-darwin-amd64.tar.gz), [windows](https://github.com/kastenhq/kubestr/releases/download/0.3.2/kubestr-0.3.2-windows-amd64.zip) or [linux](https://github.com/kastenhq/kubestr/releases/download/0.3.2/kubestr-0.3.2-linux-amd64.tar.gz). 
- Unpack the tool and make it an executable `chmod +x kubestr`.

### To discover available storage options -
- Run `./kubestr`

### To run an FIO test - 
- Run `./kubestr fio -s <storage class>`

### To check a CSI drivers snapshot and restore capabilities - 
- Run `./kubestr csicheck -s <storage class> -v <volume snapshot class>`
