## k0s status

Get k0s instance status information

```
k0s status [flags]
```

### Examples

```
The command will return information about system init, PID, k0s role, kubeconfig and similar.
```

### Options

```
  -h, --help                   help for status
  -o, --out string             sets type of output to json or yaml
      --status-socket string   Full file path to the socket file. (default "/var/lib/k0s/run/status.sock")
```

### Options inherited from parent commands

```
      --data-dir string                Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
      --debug                          Debug logging (default: false)
      --log-flush-frequency duration   Maximum number of seconds between log flushes (default 5s)
      --version version[=true]         Print version information and quit
```

### SEE ALSO

* [k0s](k0s.md)	 - k0s - Zero Friction Kubernetes

