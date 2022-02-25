# Thundernetes Traefik IngressRoute

This app is a Kubernetes controller that monitors the creation of Thundernetes GameServers and creates corresponding Kubernetes Service objects as well as Traefik IngressRoute objects.

**UNDER HEAVY CONSTRUCTION**

#### Development

```bash
go build && MIDDLEWARE_NAME=test-stripprefixregex DNS_NAME=thundernetesnoip.westus2.cloudapp.azure.com NON_TLS_ENTRYPOINT=web ./traefikingress 
```