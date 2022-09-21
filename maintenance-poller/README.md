# Maintenance Poller

Experimental hackathon project representing a Thundernetes extension that when a maintenance event comes, it will:  
1. Make the node Unschedulable (so no more GameServers are scheduled).  
2. Delete any non-Active GameServers in that node so they are not allocated. An equal number of GameServers will be recreated in nodes which are not in maintenance. 

[Azure documentation on how to handle scheduled events](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/scheduled-events)

Useful command to help monitoring the execution:

```shell
kubectl get gameservers --all-namespaces -o=custom-columns=NAME:.metadata.name,STATE:.status.state,Node:.status.nodeName
```
