## qcadmin experimental kubectl top pod

Display resource (CPU/memory) usage of pods

### Synopsis

Display resource (CPU/memory) usage of pods.

 The 'top pod' command allows you to see the resource consumption of pods.

 Due to the metrics pipeline delay, they may be unavailable for a few minutes since pod creation.

```
qcadmin experimental kubectl top pod [NAME | -l label]
```

### Examples

```
  # Show metrics for all pods in the default namespace
  kubectl top pod
  
  # Show metrics for all pods in the given namespace
  kubectl top pod --namespace=NAMESPACE
  
  # Show metrics for a given pod and its containers
  kubectl top pod POD_NAME --containers
  
  # Show metrics for the pods defined by label name=myLabel
  kubectl top pod -l name=myLabel
```

### Options

```
  -A, --all-namespaces          If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.
      --containers              If present, print usage of containers within a pod.
      --field-selector string   Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.
  -h, --help                    help for pod
      --no-headers              If present, print output without headers.
  -l, --selector string         Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.
      --sort-by string          If non-empty, sort pods list using specified field. The field can be either 'cpu' or 'memory'.
      --sum                     Print the sum of the resource usage
      --use-protocol-buffers    Enables using protocol-buffers to access Metrics API. (default true)
```

### Options inherited from parent commands

```
      --as string                      Username to impersonate for the operation. User could be a regular user or a service account in a namespace.
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                  UID to impersonate for the operation.
      --cache-dir string               Default cache directory (default "/home/runner/.kube/cache")
      --certificate-authority string   Path to a cert file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --config string                  The qcadmin config file to use
      --context string                 The name of the kubeconfig context to use
      --debug                          Prints the stack trace if an error occurs
      --disable-compression            If true, opt-out of response compression for all requests to the server
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
      --match-server-version           Require server version to match client version
  -n, --namespace string               If present, the namespace scope for this CLI request
      --password string                Password for basic authentication to the API server
      --profile string                 Name of profile to capture. One of (none|cpu|heap|goroutine|threadcreate|block|mutex) (default "none")
      --profile-output string          Name of the file to write the profile to (default "profile.pprof")
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --silent                         Run in silent mode and prevents any qcadmin log output except panics & fatals
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
      --username string                Username for basic authentication to the API server
      --warnings-as-errors             Treat warnings received from the server as errors and exit with a non-zero exit code
```

### SEE ALSO

* [qcadmin experimental kubectl top](qcadmin_experimental_kubectl_top.md)	 - Display resource (CPU/memory) usage

###### Auto generated by spf13/cobra on 26-Jul-2023
