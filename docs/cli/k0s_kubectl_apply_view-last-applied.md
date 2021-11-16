## k0s kubectl apply view-last-applied

View the latest last-applied-configuration annotations of a resource/object

### Synopsis

View the latest last-applied-configuration annotations by type/name or file.

 The default output will be printed to stdout in YAML format. You can use the -o option to change the output format.

```
k0s kubectl apply view-last-applied (TYPE [NAME | -l label] | TYPE/NAME | -f FILENAME)
```

### Examples

```
  # View the last-applied-configuration annotations by type/name in YAML
  kubectl apply view-last-applied deployment/nginx
  
  # View the last-applied-configuration annotations by file in JSON
  kubectl apply view-last-applied -f deploy.yaml -o json
```

### Options

```
      --all                Select all resources in the namespace of the specified resource types
  -f, --filename strings   Filename, directory, or URL to files that contains the last-applied-configuration annotations
  -h, --help               help for view-last-applied
  -k, --kustomize string   Process the kustomization directory. This flag can't be used together with -f or -R.
  -o, --output string      Output format. Must be one of yaml|json (default "yaml")
  -R, --recursive          Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
  -l, --selector string    Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)
```

### Options inherited from parent commands

```
      --add-dir-header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files
      --as string                        Username to impersonate for the operation
      --as-group stringArray             Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --cache-dir string                 Default cache directory (default "/home/ubuntu/.kube/cache")
      --certificate-authority string     Path to a cert file for the certificate authority
      --client-certificate string        Path to a client certificate file for TLS
      --client-key string                Path to a client key file for TLS
      --cluster string                   The name of the kubeconfig cluster to use
      --context string                   The name of the kubeconfig context to use
      --data-dir string                  Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
      --debug                            Debug logging (default: false)
      --insecure-skip-tls-verify         If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string                Path to the kubeconfig file to use for CLI requests.
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory
      --log-file string                  If non-empty, use this log file
      --log-file-max-size uint           Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --log-flush-frequency duration     Maximum number of seconds between log flushes (default 5s)
      --logtostderr                      log to standard error instead of files (default true)
      --match-server-version             Require server version to match client version
  -n, --namespace string                 If present, the namespace scope for this CLI request
      --one-output                       If true, only write logs to their native severity level (vs also writing to each lower severity level)
      --password string                  Password for basic authentication to the API server
      --profile string                   Name of profile to capture. One of (none|cpu|heap|goroutine|threadcreate|block|mutex) (default "none")
      --profile-output string            Name of the file to write the profile to (default "profile.pprof")
      --request-timeout string           The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                    The address and port of the Kubernetes API server
      --skip-headers                     If true, avoid header prefixes in the log messages
      --skip-log-headers                 If true, avoid headers when opening log files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
      --tls-server-name string           Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                     Bearer token for authentication to the API server
      --user string                      The name of the kubeconfig user to use
      --username string                  Username for basic authentication to the API server
  -v, --v Level                          number for the log level verbosity
      --version version[=true]           Print version information and quit
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
      --warnings-as-errors               Treat warnings received from the server as errors and exit with a non-zero exit code
```

### SEE ALSO

* [k0s kubectl apply](k0s_kubectl_apply.md)	 - Apply a configuration to a resource by file name or stdin

