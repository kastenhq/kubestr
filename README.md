# Kubestr

## What is it?

Kubestr is a collection of tools to discover, validate and evaluate your kubernetes storage options.

As adoption of kubernetes grows so have the persistent storage offerings that are available to users. The introduction of [CSI](https://kubernetes.io/blog/2019/01/15/container-storage-interface-ga/) (Container Storage Interface) has enabled storage providers to develop drivers with ease. In fact there are around a 100 different CSI drivers available today. Along with the existing in-tree providers, these options can make choosing the right storage difficult.

Kubestr can assist in the following ways-
- Identify the various storage options present in a cluster.
- Validate if the storage options are configured correctly.
- Evaluate the storage using common benchmarking tools like FIO.

[![asciicast](https://asciinema.org/a/7iJTbWKwdhPHNWYV00LIgx7gn.svg)](https://asciinema.org/a/7iJTbWKwdhPHNWYV00LIgx7gn)

## Resources 
Video 
* [Cloud Native Live: Introducing Kubestr – A New Way to Explore your Kubernetes Storage Options](https://youtu.be/N79NY_0aO0w)
* [Introducing Kubestr - A handy tool for Kubernetes Storage](https://youtu.be/U3Rt9vcuQdc)
* [A new way to benchmark your kubernetes storage DoK Talks #71](https://www.youtube.com/watch?v=g64eIOk_Ob4)


Blogs 
* [Benchmarking and Evaluating Your Kubernetes Storage with Kubestr](https://www.google.com/url?sa=t&rct=j&q=&esrc=s&source=web&cd=&cad=rja&uact=8&ved=2ahUKEwjpp6nn4NDyAhWXiVwKHaxHBigQFnoECAMQAQ&url=https%3A%2F%2Fblog.kasten.io%2Fbenchmarking-kubernetes-storage-with-kubestr&usg=AOvVaw1iH_P27EO2waz81znslF_Q)
* [Kubestr: The Easy Button for Validating and Debugging Your Storage in Kubernetes](https://thenewstack.io/kubestr-the-easy-button-for-validating-and-debugging-your-storage-in-kubernetes/)
* [Introducing Kubestr - A handy tool for Kubernetes Storage](https://vzilla.co.uk/vzilla-blog/introducing-kubestr-a-handy-tool-for-kubernetes-storage)


## Using Kubestr
### To install the tool -  
- Ensure that the kubernetes context is set and the cluster is accessible through your terminal. (Does [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) work?)
- Download the latest release [here](https://github.com/kastenhq/kubestr/releases/latest). 
- Unpack the tool and make it an executable `chmod +x kubestr`.

### To discover available storage options -
- Run `./kubestr`

### To run an FIO test - 
- Run `./kubestr fio -s <storage class>`
- Additional options like `--size` and `--fiofile` can be specified.
- For more information visit our [fio](https://github.com/kastenhq/kubestr/blob/master/FIO.md) page.

### To check a CSI drivers snapshot and restore capabilities - 
- Run `./kubestr csicheck -s <storage class> -v <volume snapshot class>`

## Roadmap
- In the future we plan to allow users to post their FIO results and compare to others.
