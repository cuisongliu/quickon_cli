## qcadmin experimental kubectl auth can-i

Check whether an action is allowed

### Synopsis

Check whether an action is allowed.

 VERB is a logical Kubernetes API verb like 'get', 'list', 'watch', 'delete', etc. TYPE is a Kubernetes resource. Shortcuts and groups will be resolved. NONRESOURCEURL is a partial URL that starts with "/". NAME is the name of a particular Kubernetes resource. This command pairs nicely with impersonation. See --as global flag.

```
qcadmin experimental kubectl auth can-i VERB [TYPE | TYPE/NAME | NONRESOURCEURL]
```

### Examples

```
  # Check to see if I can create pods in any namespace
  kubectl auth can-i create pods --all-namespaces
  
  # Check to see if I can list deployments in my current namespace
  kubectl auth can-i list deployments.apps
  
  # Check to see if service account "foo" of namespace "dev" can list pods
  # in the namespace "prod".
  # You must be allowed to use impersonation for the global option "--as".
  kubectl auth can-i list pods --as=system:serviceaccount:dev:foo -n prod
  
  # Check to see if I can do everything in my current namespace ("*" means all)
  kubectl auth can-i '*' '*'
  
  # Check to see if I can get the job named "bar" in namespace "foo"
  kubectl auth can-i list jobs.batch/bar -n foo
  
  # Check to see if I can read pod logs
  kubectl auth can-i get pods --subresource=log
  
  # Check to see if I can access the URL /logs/
  kubectl auth can-i get /logs/
  
  # List all allowed actions in namespace "foo"
  kubectl auth can-i --list --namespace=foo
```

### Options

```
  -A, --all-namespaces       If true, check the specified action in all namespaces.
  -h, --help                 help for can-i
      --list                 If true, prints all allowed actions.
      --no-headers           If true, prints allowed actions without headers
  -q, --quiet                If true, suppress output and just return the exit code.
      --subresource string   SubResource such as pod/log or deployment/scale
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

* [qcadmin experimental kubectl auth](qcadmin_experimental_kubectl_auth.md)	 - Inspect authorization

###### Auto generated by spf13/cobra on 26-Jul-2023
