/*
Copyright 2020 Mirantis, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controller

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"golang.org/x/sync/errgroup"

	"github.com/pkg/errors"

	"github.com/k0sproject/k0s/internal/util"
	config "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/certificate"
	"github.com/k0sproject/k0s/pkg/constant"

	"github.com/sirupsen/logrus"
)

var (
	kubeconfigTemplate = template.Must(template.New("kubeconfig").Parse(`
apiVersion: v1
clusters:
- cluster:
    server: {{.URL}}
    certificate-authority-data: {{.CACert}}
  name: local
contexts:
- context:
    cluster: local
    namespace: default
    user: user
  name: Default
current-context: Default
kind: Config
preferences: {}
users:
- name: user
  user:
    client-certificate-data: {{.ClientCert}}
    client-key-data: {{.ClientKey}}
`))
)

// Certificates is the Component implementation to manage all k0s certs
type Certificates struct {
	CACert      string
	CertManager certificate.Manager
	ClusterSpec *config.ClusterSpec
	K0sVars     constant.CfgVars
}

// Init initializes the certificate component
func (c *Certificates) Init() error {

	eg, _ := errgroup.WithContext(context.Background())
	// Common CA
	caCertPath := filepath.Join(c.K0sVars.CertRootDir, "ca.crt")
	caCertKey := filepath.Join(c.K0sVars.CertRootDir, "ca.key")

	if err := c.CertManager.EnsureCA("ca", "kubernetes-ca"); err != nil {
		return err
	}

	// We need CA cert loaded to generate client configs
	logrus.Debugf("CA key and cert exists, loading")
	cert, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read ca cert")
	}
	c.CACert = string(cert)

	eg.Go(func() error {
		// Front proxy CA
		if err := c.CertManager.EnsureCA("front-proxy-ca", "kubernetes-front-proxy-ca"); err != nil {
			return err
		}

		proxyCertPath, proxyCertKey := filepath.Join(c.K0sVars.CertRootDir, "front-proxy-ca.crt"), filepath.Join(c.K0sVars.CertRootDir, "front-proxy-ca.key")

		proxyClientReq := certificate.Request{
			Name:   "front-proxy-client",
			CN:     "front-proxy-client",
			O:      "front-proxy-client",
			CACert: proxyCertPath,
			CAKey:  proxyCertKey,
		}
		_, err := c.CertManager.EnsureCertificate(proxyClientReq, constant.ApiserverUser)

		return err
	})

	eg.Go(func() error {
		// admin cert & kubeconfig
		adminReq := certificate.Request{
			Name:   "admin",
			CN:     "kubernetes-admin",
			O:      "system:masters",
			CACert: caCertPath,
			CAKey:  caCertKey,
		}
		adminCert, err := c.CertManager.EnsureCertificate(adminReq, "root")
		if err != nil {
			return err
		}
		if err := kubeConfig(c.K0sVars.AdminKubeConfigPath, "https://localhost:6443", c.CACert, adminCert.Cert, adminCert.Key, "root"); err != nil {
			return err
		}

		return generateKeyPair("sa", c.K0sVars, constant.ApiserverUser)
	})

	eg.Go(func() error {
		// konnectivity kubeconfig
		konnectivityReq := certificate.Request{
			Name:   "konnectivity",
			CN:     "kubernetes-konnectivity",
			O:      "system:masters", // TODO: We need to figure out if konnectivity really needs superpowers
			CACert: caCertPath,
			CAKey:  caCertKey,
		}
		konnectivityCert, err := c.CertManager.EnsureCertificate(konnectivityReq, constant.KonnectivityServerUser)
		if err != nil {
			return err
		}
		if err := kubeConfig(c.K0sVars.KonnectivityKubeConfigPath, "https://localhost:6443", c.CACert, konnectivityCert.Cert, konnectivityCert.Key, constant.KonnectivityServerUser); err != nil {
			return err
		}

		return nil
	})

	eg.Go(func() error {
		ccmReq := certificate.Request{
			Name:   "ccm",
			CN:     "system:kube-controller-manager",
			O:      "system:kube-controller-manager",
			CACert: caCertPath,
			CAKey:  caCertKey,
		}
		ccmCert, err := c.CertManager.EnsureCertificate(ccmReq, constant.ApiserverUser)

		if err != nil {
			return err
		}

		return kubeConfig(filepath.Join(c.K0sVars.CertRootDir, "ccm.conf"), "https://localhost:6443", c.CACert, ccmCert.Cert, ccmCert.Key, constant.ApiserverUser)
	})

	eg.Go(func() error {
		schedulerReq := certificate.Request{
			Name:   "scheduler",
			CN:     "system:kube-scheduler",
			O:      "system:kube-scheduler",
			CACert: caCertPath,
			CAKey:  caCertKey,
		}
		schedulerCert, err := c.CertManager.EnsureCertificate(schedulerReq, constant.SchedulerUser)
		if err != nil {
			return err
		}

		return kubeConfig(filepath.Join(c.K0sVars.CertRootDir, "scheduler.conf"), "https://localhost:6443", c.CACert, schedulerCert.Cert, schedulerCert.Key, constant.SchedulerUser)
	})

	eg.Go(func() error {
		kubeletClientReq := certificate.Request{
			Name:   "apiserver-kubelet-client",
			CN:     "apiserver-kubelet-client",
			O:      "system:masters",
			CACert: caCertPath,
			CAKey:  caCertKey,
		}
		_, err := c.CertManager.EnsureCertificate(kubeletClientReq, constant.ApiserverUser)
		return err
	})

	hostnames := []string{
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster",
		"kubernetes.svc.cluster.local",
		"127.0.0.1",
		"localhost",
	}

	hostnames = append(hostnames, c.ClusterSpec.API.Sans()...)

	internalAPIAddress, err := c.ClusterSpec.Network.InternalAPIAddresses()
	if err != nil {
		return err
	}
	hostnames = append(hostnames, internalAPIAddress...)

	eg.Go(func() error {
		serverReq := certificate.Request{
			Name:      "server",
			CN:        "kubernetes",
			O:         "kubernetes",
			CACert:    caCertPath,
			CAKey:     caCertKey,
			Hostnames: hostnames,
		}
		_, err = c.CertManager.EnsureCertificate(serverReq, constant.ApiserverUser)

		return err
	})

	eg.Go(func() error {
		apiReq := certificate.Request{
			Name:      "k0s-api",
			CN:        "k0s-api",
			O:         "kubernetes",
			CACert:    caCertPath,
			CAKey:     caCertKey,
			Hostnames: hostnames,
		}
		// TODO Not sure about the user...
		_, err := c.CertManager.EnsureCertificate(apiReq, constant.ApiserverUser)
		return err
	})

	return eg.Wait()
}

// Run does nothing, the cert component only needs to be initialized
func (c *Certificates) Run() error {
	return nil
}

// Stop does nothing, the cert component is not constantly running
func (c *Certificates) Stop() error {
	return nil
}

func kubeConfig(dest, url, caCert, clientCert, clientKey, owner string) error {
	if util.FileExists(dest) {
		return chownFile(dest, owner, constant.CertSecureMode)
	}
	data := struct {
		URL        string
		CACert     string
		ClientCert string
		ClientKey  string
	}{
		URL:        url,
		CACert:     base64.StdEncoding.EncodeToString([]byte(caCert)),
		ClientCert: base64.StdEncoding.EncodeToString([]byte(clientCert)),
		ClientKey:  base64.StdEncoding.EncodeToString([]byte(clientKey)),
	}

	output, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC, constant.CertSecureMode)
	if err != nil {
		return err
	}
	defer output.Close()

	if err = kubeconfigTemplate.Execute(output, &data); err != nil {
		return err
	}

	return chownFile(output.Name(), owner, constant.CertSecureMode)
}

func chownFile(file, owner string, permissions os.FileMode) error {
	// Chown the file properly for the owner
	uid, _ := util.GetUID(owner)
	err := os.Chown(file, uid, -1)
	if err != nil && os.Geteuid() == 0 {
		return err
	}
	err = util.Chmod(file, permissions)
	if err != nil && os.Geteuid() == 0 {
		return err
	}

	return nil
}

func generateKeyPair(name string, k0sVars constant.CfgVars, owner string) error {
	keyFile := filepath.Join(k0sVars.CertRootDir, fmt.Sprintf("%s.key", name))
	pubFile := filepath.Join(k0sVars.CertRootDir, fmt.Sprintf("%s.pub", name))

	if util.FileExists(keyFile) && util.FileExists(pubFile) {
		return chownFile(keyFile, owner, constant.CertSecureMode)
	}

	reader := rand.Reader
	bitSize := 2048

	key, err := rsa.GenerateKey(reader, bitSize)
	if err != nil {
		return err
	}

	var privateKey = &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	outFile, err := os.OpenFile(keyFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, constant.CertSecureMode)
	if err != nil {
		return err
	}
	defer outFile.Close()

	err = chownFile(keyFile, owner, constant.CertSecureMode)
	if err != nil {
		return err
	}

	err = pem.Encode(outFile, privateKey)
	if err != nil {
		return err
	}

	// note to the next reader: key.Public() != key.PublicKey
	pubBytes, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return err
	}

	var pemkey = &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}

	pemfile, err := os.Create(pubFile)
	if err != nil {
		return err
	}
	defer pemfile.Close()

	err = pem.Encode(pemfile, pemkey)
	if err != nil {
		return err
	}

	return nil
}

// Health-check interface
func (c *Certificates) Healthy() error { return nil }
