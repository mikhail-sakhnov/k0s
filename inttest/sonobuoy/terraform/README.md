# Sonobuoy 

This is a TF configuration to easily setup a K0s cluster with AWS and start sonobuoy.
Requirements:

1. Terraform
2. AWS credentials

To provision required setup follow next steps:

```
$ terraform init
$ terraform apply
module.k0s-sonobuoy.data.aws_ami.ubuntu: Refreshing state...
module.k0s-sonobuoy.tls_private_key.k8s-conformance-key: Creating...
module.k0s-sonobuoy.tls_private_key.k8s-conformance-key: Creation complete after 1s [id=07fae4a3e454177b6156c3342d6d92008426d703]
module.k0s-sonobuoy.local_file.aws_private_pem: Creating...
...
Apply complete! Resources: 17 added, 0 changed, 0 destroyed.

Outputs:

controller_ip = [
  "54.73.141.241",
]
```

To get sonobuoy results you have to SSH into the controller:

```
$ ssh -i ../../terraform/test-cluster/aws_private.pem ubuntu@[controller_ip]

ubuntu@controller-0:~$ export KUBECONFIG=/var/lib/k0s/pki/admin.conf
ubuntu@controller-0:~$ sonobuoy status
ubuntu@controller-0:~$ export KUBECONFIG=/var/lib/k0s/pki/admin.conf
ubuntu@controller-0:~$ sonobuoy status
         PLUGIN     STATUS   RESULT   COUNT
            e2e    running                1
   systemd-logs   complete                2
   systemd-logs    running                1

Sonobuoy is still running. Runs can take up to 60 minutes.
```