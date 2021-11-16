## k0s kubectl create

Create a resource from a file or from stdin

### Synopsis

Create a resource from a file or from stdin.

 JSON and YAML formats are accepted.

```
k0s kubectl create -f FILENAME
```

### Examples

```
  # Create a pod using the data in pod.json
  kubectl create -f ./pod.json
  
  # Create a pod based on the JSON passed into stdin
  cat pod.json | kubectl create -f -
  
  # Edit the data in docker-registry.yaml in JSON then create the resource using the edited data
  kubectl create -f docker-registry.yaml --edit -o json
```

### Options

```
      --allow-missing-template-keys    If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats. (default true)
      --dry-run string[="unchanged"]   Must be "none", "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource. (default "none")
      --edit                           Edit the API resource before creating
      --field-manager string           Name of the manager used to track field ownership. (default "kubectl-create")
  -f, --filename strings               Filename, directory, or URL to files to use to create the resource
  -h, --help                           help for create
  -k, --kustomize string               Process the kustomization directory. This flag can't be used together with -f or -R.
  -o, --output string                  Output format. One of: json|yaml|name|go-template|go-template-file|template|templatefile|jsonpath|jsonpath-as-json|jsonpath-file.
      --raw string                     Raw URI to POST to the server.  Uses the transport specified by the kubeconfig file.
  -R, --recursive                      Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
      --save-config                    If true, the configuration of current object will be saved in its annotation. Otherwise, the annotation will be unchanged. This flag is useful when you want to perform kubectl apply on this object in the future.
  -l, --selector string                Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)
      --show-managed-fields            If true, keep the managedFields when printing objects in JSON or YAML format.
      --template string                Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
      --validate                       If true, use a schema to validate the input before sending it (default true)
      --windows-line-endings           Only relevant if --edit=true. Defaults to the line ending native to your platform.
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
* [k0s kubectl create clusterrole](k0s_kubectl_create_clusterrole.md)	 - Create a cluster role
* [k0s kubectl create clusterrolebinding](k0s_kubectl_create_clusterrolebinding.md)	 - Create a cluster role binding for a particular cluster role
* [k0s kubectl create configmap](k0s_kubectl_create_configmap.md)	 - Create a config map from a local file, directory or literal value
* [k0s kubectl create cronjob](k0s_kubectl_create_cronjob.md)	 - Create a cron job with the specified name
* [k0s kubectl create deployment](k0s_kubectl_create_deployment.md)	 - Create a deployment with the specified name
* [k0s kubectl create ingress](k0s_kubectl_create_ingress.md)	 - Create an ingress with the specified name
* [k0s kubectl create job](k0s_kubectl_create_job.md)	 - Create a job with the specified name
* [k0s kubectl create namespace](k0s_kubectl_create_namespace.md)	 - Create a namespace with the specified name
* [k0s kubectl create poddisruptionbudget](k0s_kubectl_create_poddisruptionbudget.md)	 - Create a pod disruption budget with the specified name
* [k0s kubectl create priorityclass](k0s_kubectl_create_priorityclass.md)	 - Create a priority class with the specified name
* [k0s kubectl create quota](k0s_kubectl_create_quota.md)	 - Create a quota with the specified name
* [k0s kubectl create role](k0s_kubectl_create_role.md)	 - Create a role with single rule
* [k0s kubectl create rolebinding](k0s_kubectl_create_rolebinding.md)	 - Create a role binding for a particular role or cluster role
* [k0s kubectl create secret](k0s_kubectl_create_secret.md)	 - Create a secret using specified subcommand
* [k0s kubectl create service](k0s_kubectl_create_service.md)	 - Create a service using a specified subcommand
* [k0s kubectl create serviceaccount](k0s_kubectl_create_serviceaccount.md)	 - Create a service account with the specified name

