module "k0s-sonobuoy" {
  source       = "../../terraform/test-cluster"
  cluster_name = "sonobuoy_test"
}

output "controller_ip" {
  value = module.k0s-sonobuoy.controller_external_ip
}

resource "null_resource" "controller" {
  depends_on = [module.k0s-sonobuoy]
  connection {
    type        = "ssh"
    private_key = module.k0s-sonobuoy.controller_pem.content
    host        = module.k0s-sonobuoy.controller_external_ip[0]
    agent       = true
    user        = "ubuntu"
  }

  provisioner "file" {
    source      = "./gh.sh"
    destination = "/home/ubuntu/gh.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "sudo chmod +x /home/ubuntu/gh.sh",
      "sudo /home/ubuntu/gh.sh",
      "sudo nohup k0s server --enable-worker >/home/ubuntu/k0s-master.log 2>&1 &",
      "echo 'Wait 10 seconds for cluster to start!!!'",
      "sleep 10"
    ]
  }
}

resource "null_resource" "configure_worker1" {
  depends_on = [null_resource.controller]
  connection {
    type        = "ssh"
    private_key = module.k0s-sonobuoy.controller_pem.content
    host        = module.k0s-sonobuoy.worker_external_ip[0]
    agent       = true
    user        = "ubuntu"
  }


  provisioner "file" {
    source      = module.k0s-sonobuoy.controller_pem.filename
    destination = "/home/ubuntu/.ssh/id_rsa"
  }

  provisioner "file" {
    source      = "./startworker.sh"
    destination = "/home/ubuntu/startworker.sh"
  }

  provisioner "file" {
    source      = "./gh.sh"
    destination = "/home/ubuntu/gh.sh"
  }


  provisioner "remote-exec" {
    inline = [
      "sudo chmod +x /home/ubuntu/gh.sh",
      "sudo /home/ubuntu/gh.sh",
      "sudo chmod +x /home/ubuntu/startworker.sh ",
      "sudo /home/ubuntu/startworker.sh ${module.k0s-sonobuoy.controller_external_ip[0]}",
      "echo 'Wait 10 seconds for worker to start!!!'",
      "sleep 10",
    ]
  }
}

resource "null_resource" "configure_worker2" {
  depends_on = [null_resource.controller]
  connection {
    type        = "ssh"
    private_key = module.k0s-sonobuoy.controller_pem.content
    host        = module.k0s-sonobuoy.worker_external_ip[1]
    agent       = true
    user        = "ubuntu"
  }

  provisioner "file" {
    source      = module.k0s-sonobuoy.controller_pem.filename
    destination = "/home/ubuntu/.ssh/id_rsa"
  }

  provisioner "file" {
    source      = "./startworker.sh"
    destination = "/home/ubuntu/startworker.sh"
  }

  provisioner "file" {
    source      = "./gh.sh"
    destination = "/home/ubuntu/gh.sh"
  }


  provisioner "remote-exec" {
    inline = [
      "sudo chmod +x /home/ubuntu/gh.sh",
      "sudo /home/ubuntu/gh.sh",
      "sudo chmod +x /home/ubuntu/startworker.sh",
      "sudo /home/ubuntu/startworker.sh  ${module.k0s-sonobuoy.controller_external_ip[0]}",
      "echo 'Wait 10 seconds for worker to start!!!'",
      "sleep 10",
    ]
  }
}


resource "null_resource" "sonobuoy" {
  depends_on = [null_resource.configure_worker2]
  connection {
    type        = "ssh"
    private_key = module.k0s-sonobuoy.controller_pem.content
    host        = module.k0s-sonobuoy.controller_external_ip[0]
    agent       = true
    user        = "ubuntu"
  }

  provisioner "remote-exec" {
    inline = [
     "wget https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.19.0/sonobuoy_0.19.0_linux_amd64.tar.gz",
      "tar -xvf sonobuoy_0.19.0_linux_amd64.tar.gz",
      "sudo mv sonobuoy /usr/local/bin",
      "sudo chmod +x /usr/local/bin/sonobuoy",
      "KUBECONFIG=/var/lib/k0s/pki/admin.conf sonobuoy run"
      ]
  }
}