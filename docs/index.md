---
layout: home
title: Kubestr 
subtitle: Explore your Kubernetes storage options
baseurl: "kubestr.io" 
---

# Kubestr

## What is it?

Kubestr is a collection of tools to discover, validate and evaluate your kubernetes storage options.

As adoption of Kubernetes grows so have the persistent storage offerings that are available to users. The introduction of [CSI](https://kubernetes.io/blog/2019/01/15/container-storage-interface-ga/) (Container Storage Interface) has enabled storage providers to develop drivers with ease. In fact there are around a 100 different CSI drivers available today. Along with the existing in-tree providers, these options can make choosing the right storage difficult.

Kubestr can assist in the following ways-
* Identify the various storage options present in a cluster.
* Validate if the storage options are configured correctly.
* Evaluate the storage using common benchmarking tools like FIO.
* View the contents of a PersistentVolumeClaim in a graphical filesystem browser.


<script id="asciicast-7iJTbWKwdhPHNWYV00LIgx7gn" src="https://asciinema.org/a/7iJTbWKwdhPHNWYV00LIgx7gn.js" async></script>

## Resources
### Video
* [Cloud Native Live: Introducing Kubestr – A New Way to Explore your Kubernetes Storage Options](https://youtu.be/N79NY_0aO0w)
* [Introducing Kubestr - A handy tool for Kubernetes Storage](https://youtu.be/U3Rt9vcuQdc)
* [A new way to benchmark your kubernetes storage DoK Talks #71](https://www.youtube.com/watch?v=g64eIOk_Ob4)

### Blogs
* [Benchmarking and Evaluating Your Kubernetes Storage with Kubestr](https://blog.kasten.io/benchmarking-kubernetes-storage-with-kubestr)
* [Kubestr: The Easy Button for Validating and Debugging Your Storage in Kubernetes](https://thenewstack.io/kubestr-the-easy-button-for-validating-and-debugging-your-storage-in-kubernetes/)
* [Introducing Kubestr - A handy tool for Kubernetes Storage](https://vzilla.co.uk/vzilla-blog/introducing-kubestr-a-handy-tool-for-kubernetes-storage)


## Using Kubestr
### To install the tool -  
- Ensure that the Kubernetes context is set and the cluster is accessible through your terminal. (Does [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) work?)
- Download the latest release [here](https://github.com/kastenhq/kubestr/releases/latest). 
- Unpack the tool and make it an executable `chmod +x kubestr`.

### To discover available storage options -
- Run `./kubestr`

### To run an FIO test - 
- Run `./kubestr fio -s <storage class>`
- Additional options like `--size` and `--fiofile` can be specified.
- For more information visit our [fio]({{ site.baseurl }}{% link fio.md  %}) page.

### To check a CSI drivers snapshot and restore capabilities - 
- Run `./kubestr csicheck -s <storage class> -v <volume snapshot class>`

### To view the contents of a PersistentVolumeClaim in a graphical filesystem browser -
- Run `./kubestr browse pvc <pvc name> -n <namespace> -v <volume snapshot class>`
- Additional flag `--show-tree` can be specified to view contents on CLI.

### To view the contents of a VolumeSnapshot in a graphical filesystem browser -
- Run `./kubestr browse snapshot <snapshot name> -n <namespace> -s <storage class>`
- Additional flag `--show-tree` can be specified to view contents on CLI.

### To restore files from a VolumeSnapshot using graphical filesystem browser -
- Run `./kubestr file-restore --fromSnapshot <snapshot name> -n <namespace>`
<br> This restores to a PersistentVolumeClaim specified by the VolumeSnapshot.
- Optionally, run `./kubestr file-restore --fromSnapshot <snapshot name> -n <namespace> [--toPVC <pvc name>]`
<br> to restore to a specified PersistentVolumeClaim.
- Additionally, an option `--path` can be specified to restore a file from given path using CLI.

## Roadmap
- For future work please refer to our GitHub issues [page](https://github.com/kastenhq/kubestr/issues).
Feel free to post here for any feature requests.


## Contributing
- Forking and contributing to this project are very welcome. Please visit our github page [here](https://github.com/kastenhq/kubestr).

