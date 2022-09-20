# Maintenance Poller

https://learn.microsoft.com/en-us/azure/virtual-machines/linux/scheduled-events

```shell
kubectl get gameservers --all-namespaces -o=custom-columns=NAME:.metadata.name,STATE:.status.state,Node:.status.nodeName
```
