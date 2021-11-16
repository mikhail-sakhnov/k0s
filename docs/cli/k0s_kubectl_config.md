## k0s kubectl config

Modify kubeconfig files

### Synopsis

Modify kubeconfig files using subcommands like "kubectl config set current-context my-context"

 The loading order follows these rules:

  1.  If the --kubeconfig flag is set, then only that file is loaded. The flag may only be set once and no merging takes place.
  2.  If $KUBECONFIG environment variable is set, then it is used as a list of paths (normal path delimiting rules for your system). These paths are merged. When a value is modified, it is modified in the file that defines the stanza. When a value is created, it is created in the first file that exists. If no files in the chain exist, then it creates the last file in the list.
  3.  Otherwise, ${HOME}/.kube/config is used and no merging takes place.

```
k0s kubectl config SUBCOMMAND
```

### Options

```
  -h, --help   help for config
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

* [k0s kubectl](k0s_kubectl.md)	 - kubectl controls the Kubernetes cluster manager
* [k0s kubectl config current-context](k0s_kubectl_config_current-context.md)	 - Display the current-context
* [k0s kubectl config delete-cluster](k0s_kubectl_config_delete-cluster.md)	 - Delete the specified cluster from the kubeconfig
* [k0s kubectl config delete-context](k0s_kubectl_config_delete-context.md)	 - Delete the specified context from the kubeconfig
* [k0s kubectl config delete-user](k0s_kubectl_config_delete-user.md)	 - Delete the specified user from the kubeconfig
* [k0s kubectl config get-clusters](k0s_kubectl_config_get-clusters.md)	 - Display clusters defined in the kubeconfig
* [k0s kubectl config get-contexts](k0s_kubectl_config_get-contexts.md)	 - Describe one or many contexts
* [k0s kubectl config get-users](k0s_kubectl_config_get-users.md)	 - Display users defined in the kubeconfig
* [k0s kubectl config rename-context](k0s_kubectl_config_rename-context.md)	 - Rename a context from the kubeconfig file
* [k0s kubectl config set](k0s_kubectl_config_set.md)	 - Set an individual value in a kubeconfig file
* [k0s kubectl config set-cluster](k0s_kubectl_config_set-cluster.md)	 - Set a cluster entry in kubeconfig
* [k0s kubectl config set-context](k0s_kubectl_config_set-context.md)	 - Set a context entry in kubeconfig
* [k0s kubectl config set-credentials](k0s_kubectl_config_set-credentials.md)	 - Set a user entry in kubeconfig
* [k0s kubectl config unset](k0s_kubectl_config_unset.md)	 - Unset an individual value in a kubeconfig file
* [k0s kubectl config use-context](k0s_kubectl_config_use-context.md)	 - Set the current-context in a kubeconfig file
* [k0s kubectl config view](k0s_kubectl_config_view.md)	 - Display merged kubeconfig settings or a specified kubeconfig file

