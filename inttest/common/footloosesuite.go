/*
Copyright 2022 k0s authors

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

package common

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"text/template"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/file"
	apclient "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2/clientset"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	extclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	"github.com/go-openapi/jsonpointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/weaveworks/footloose/pkg/cluster"
	"github.com/weaveworks/footloose/pkg/config"
	"go.uber.org/multierr"
	"golang.org/x/sync/errgroup"
)

const (
	controllerNodeNameFormat   = "controller%d"
	workerNodeNameFormat       = "worker%d"
	lbNodeNameFormat           = "lb%d"
	etcdNodeNameFormat         = "etcd%d"
	updateServerNodeNameFormat = "updateserver%d"

	defaultK0sBinaryFullPath = "/usr/local/bin/k0s"
	k0sBindMountFullPath     = "/dist/k0s"
	k0sNewBindMountFullPath  = "/dist/k0s-new"

	defaultK0sUpdateVersion = "v0.0.0"
)

// FootlooseSuite defines all the common stuff we need to be able to run k0s testing on footloose.
type FootlooseSuite struct {
	suite.Suite

	/* config knobs (initialized via `initializeDefaults`) */

	LaunchMode                   LaunchMode
	ControllerCount              int
	ControllerUmask              int
	ExtraVolumes                 []config.Volume
	K0sFullPath                  string
	AirgapImageBundleMountPoints []string
	K0sAPIExternalPort           int
	KonnectivityAdminPort        int
	KonnectivityAgentPort        int
	KubeAPIExternalPort          int
	WithExternalEtcd             bool
	WithLB                       bool
	WorkerCount                  int
	WithUpdateServer             bool
	K0sUpdateVersion             string
	ControllerNetworks           []string
	WorkerNetworks               []string

	/* context and cancellation */

	ctxPtr atomic.Pointer[suiteCtx]

	/* footloose cluster setup */

	clusterDir     string
	clusterConfig  config.Config
	cluster        *cluster.Cluster
	launchDelegate launchDelegate

	dataDirOpt string // Data directory option of first controller, required to fetch the cluster state
}

type suiteCtx struct {
	ctx  context.Context
	stop func()
}

// initializeDefaults initializes any unset configuration knobs to their defaults.
func (s *FootlooseSuite) initializeDefaults() {
	if s.K0sFullPath == "" {
		s.K0sFullPath = defaultK0sBinaryFullPath
	}
	if s.K0sAPIExternalPort == 0 {
		s.K0sAPIExternalPort = 9443
	}
	if s.KonnectivityAdminPort == 0 {
		s.KonnectivityAdminPort = 8133
	}
	if s.KonnectivityAgentPort == 0 {
		s.KonnectivityAgentPort = 8132
	}
	if s.KubeAPIExternalPort == 0 {
		s.KubeAPIExternalPort = 6443
	}
	if s.LaunchMode == "" {
		s.LaunchMode = LaunchModeStandalone
	}

	s.K0sUpdateVersion = os.Getenv("K0S_UPDATE_TO_VERSION")
	if s.K0sUpdateVersion == "" {
		s.K0sUpdateVersion = defaultK0sUpdateVersion
	}

	switch s.LaunchMode {
	case LaunchModeStandalone:
		s.launchDelegate = &standaloneLaunchDelegate{s.K0sFullPath, s.ControllerUmask}
	case LaunchModeOpenRC:
		s.launchDelegate = &openRCLaunchDelegate{s.K0sFullPath}
	default:
		s.Require().Fail("Unknown launch mode", s.LaunchMode)
	}
}

// SetupSuite does all the setup work, namely boots up footloose cluster.
func (s *FootlooseSuite) SetupSuite() {
	t := s.T()

	s.initializeDefaults()

	ctx, cancel := newSuiteContext(t)
	var cleanupTasks sync.WaitGroup
	sctx := suiteCtx{ctx, func() {
		cancel()
		cleanupTasks.Wait()
	}}

	if !s.ctxPtr.CompareAndSwap(nil, &sctx) {
		s.Require().Fail("Failed to install suite context")
	}

	if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
		t.Logf("test teardown deadline: %s", deadline)
	} else {
		t.Log("test suite has no deadline")
	}

	if err := s.initializeFootlooseCluster(); err != nil {
		s.FailNow("failed to initialize footloose cluster", err)
	}

	// perform a cleanup whenever the suite's context is canceled
	cleanupTasks.Add(1)
	go func() {
		defer cleanupTasks.Done()
		<-ctx.Done()

		t.Logf("Cleaning up")

		// Replace the done context with a fresh one.
		ctx, cancel := newSuiteContext(t)
		defer cancel()
		if s.ctxPtr.CompareAndSwap(&sctx, &suiteCtx{ctx, cleanupTasks.Wait}) {
			if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
				t.Logf("Test cleanup deadline: %s", deadline)
			} else {
				t.Log("Test cleanup has no deadline")
			}
		} else {
			t.Log("Failed to replace suite context during cleanup")
		}

		s.cleanupSuite(t)
	}()

	// set up signal handler so we teardown on Interrupt or SIGTERM
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		signal := <-c
		assert.Fail(t, "Signal received", "%s", signal)
		s.TearDownSuite()
		os.Exit(1)
	}()

	s.waitForSSH()

	if s.WithLB {
		s.startHAProxy()
	}
}

// waitForSSH waits to get a SSH connection to all footloose machines defined as part of the test suite.
// Each node is tried in parallel for ~30secs max
func (s *FootlooseSuite) waitForSSH() {
	nodes := []string{}
	for i := 0; i < s.ControllerCount; i++ {
		nodes = append(nodes, s.ControllerNode(i))
	}
	for i := 0; i < s.WorkerCount; i++ {
		nodes = append(nodes, s.WorkerNode(i))
	}
	if s.WithLB {
		nodes = append(nodes, s.LBNode())
	}

	s.T().Logf("Waiting for SSH connections to %d nodes: %v", len(nodes), nodes)

	g, ctx := errgroup.WithContext(s.Context())
	for _, node := range nodes {
		nodeName := node
		g.Go(func() error {
			return wait.PollUntilWithContext(ctx, 1*time.Second, func(ctx context.Context) (bool, error) {
				ssh, err := s.SSH(nodeName)
				if err != nil {
					return false, nil
				}
				defer ssh.Disconnect()

				err = ssh.Exec(ctx, "hostname", SSHStreams{})
				if err != nil {
					return false, nil
				}

				s.T().Logf("SSH connection to %s successful", nodeName)
				return true, nil
			})
		})
	}

	s.Require().NoError(g.Wait(), "Failed to ssh into all nodes")
}

// Context returns this suite's context, which should be passed to all blocking operations.
func (s *FootlooseSuite) Context() context.Context {
	sctx := s.ctxPtr.Load()
	s.Require().NotNil(sctx, "No suite context installed")
	return sctx.ctx
}

// ControllerNode gets the node name of given controller index
func (s *FootlooseSuite) ControllerNode(idx int) string {
	return fmt.Sprintf(controllerNodeNameFormat, idx)
}

// WorkerNode gets the node name of given worker index
func (s *FootlooseSuite) WorkerNode(idx int) string {
	return fmt.Sprintf(workerNodeNameFormat, idx)
}

func (s *FootlooseSuite) LBNode() string {
	if !s.WithLB {
		s.FailNow("can't get load balancer node name because it's not enabled for this suite")
	}
	return fmt.Sprintf(lbNodeNameFormat, 0)
}

func (s *FootlooseSuite) ExternalEtcdNode() string {
	if !s.WithExternalEtcd {
		s.FailNow("can't get external node name because it's not enabled for this suite")
	}
	return fmt.Sprintf(etcdNodeNameFormat, 0)
}

// TearDownSuite is called by testify at the very end of the suite's run.
// It cancels the suite's context in order to free the suite's resources.
func (s *FootlooseSuite) TearDownSuite() {
	sctx := s.ctxPtr.Load()
	s.Require().NotNil(sctx, "No suite context installed")
	sctx.stop()
}

// cleanupSuite does the cleanup work, namely destroy the footloose machines.
// Intended to be called after the suite's context has been canceled.
func (s *FootlooseSuite) cleanupSuite(t *testing.T) {
	ctx := s.Context()
	var wg sync.WaitGroup

	tmpDir := os.TempDir()

	if t.Failed() && s.ControllerCount > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.dumpClusterState(t, ctx, filepath.Join(tmpDir, "cluster-state.tar"))
		}()
	}

	machines, err := s.InspectMachines(nil)
	if err != nil {
		t.Logf("Failed to inspect machines: %s", err.Error())
		machines = nil
	}

	for _, m := range machines {
		node := m.Hostname()
		if strings.HasPrefix(node, "lb") {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			s.dumpNodeLogs(t, ctx, node, tmpDir)
		}()
	}

	wg.Wait()

	if keepEnvironment(t) {
		t.Logf("footloose cluster left intact for debugging; needs to be manually cleaned up with: footloose delete --config %s", path.Join(s.clusterDir, "footloose.yaml"))
		return
	}

	if err := s.cluster.Delete(); err != nil {
		t.Logf("Failed to delete footloose cluster: %s", err.Error())
	}

	cleanupClusterDir(t, s.clusterDir)
}

func (s *FootlooseSuite) dumpClusterState(t *testing.T, ctx context.Context, filePath string) {
	node := s.ControllerNode(0)

	ssh, err := s.SSH(node)
	if err != nil {
		t.Logf("Failed to ssh into node %s to dump cluster state: %s", node, err.Error())
		return
	}
	defer ssh.Disconnect()

	var cmdBuf strings.Builder
	cmdBuf.WriteString(s.K0sFullPath)
	if s.dataDirOpt != "" {
		cmdBuf.WriteRune(' ')
		cmdBuf.WriteString(s.dataDirOpt)
	}
	cmdBuf.WriteString(` kc cluster-info dump -A --output-directory="${TMPDIR-/tmp}"/cluster-state`)
	cmd := cmdBuf.String()

	if err := ssh.Exec(ctx, cmd, SSHStreams{}); err != nil {
		t.Logf("Failed to dump cluster state on node %s: %s", node, err.Error())
		return
	}

	err = file.WriteAtomically(filePath, 0644, func(unbuffered io.Writer) error {
		w := bufio.NewWriter(unbuffered)
		err := ssh.Exec(ctx, `tar c -C "${TMPDIR-/tmp}" cluster-state`, SSHStreams{Out: w})
		if err != nil {
			return err
		}
		return w.Flush()
	})
	if err != nil {
		t.Logf("Failed to dump cluster state into %s: %s", filePath, err.Error())
		return
	}

	t.Logf("Dumped cluster state into %s", filePath)
}

func (s *FootlooseSuite) dumpNodeLogs(t *testing.T, ctx context.Context, node, dir string) {
	ssh, err := s.SSH(node)
	if err != nil {
		t.Logf("Failed to ssh into node %s to get logs: %s", node, err.Error())
		return
	}
	defer ssh.Disconnect()

	outPath := filepath.Join(dir, fmt.Sprintf("%s.out.log", node))
	errPath := filepath.Join(dir, fmt.Sprintf("%s.err.log", node))

	err = func() (err error) {
		type log struct {
			path   string
			writer io.Writer
		}

		outLog, errLog := log{path: outPath}, log{path: errPath}
		for _, log := range []*log{&outLog, &errLog} {
			file, err := os.Create(log.path)
			if err != nil {
				t.Logf("Failed to create log file: %s", err.Error())
				continue
			}

			defer multierr.AppendInvoke(&err, multierr.Close(file))
			buf := bufio.NewWriter(file)
			defer func() {
				if err == nil {
					err = buf.Flush()
				}
			}()
			log.writer = buf
		}

		return s.launchDelegate.ReadK0sLogs(ctx, ssh, outLog.writer, errLog.writer)
	}()
	if err != nil {
		t.Logf("Failed to save k0s logs from node %s: %s", node, err.Error())
	}

	nonEmptyPaths := make([]string, 0, 2)
	for _, path := range []string{outPath, errPath} {
		stat, err := os.Stat(path)
		if err != nil {
			continue
		}
		if stat.Size() == 0 {
			_ = os.Remove(path)
			continue
		}

		nonEmptyPaths = append(nonEmptyPaths, path)
	}

	if len(nonEmptyPaths) > 0 {
		t.Logf("Saved k0s logs of node %s to %s", node, strings.Join(nonEmptyPaths, " and "))
	}
}

const keepAfterTestsEnv = "K0S_KEEP_AFTER_TESTS"

func keepEnvironment(t *testing.T) bool {
	keepAfterTests := os.Getenv(keepAfterTestsEnv)
	switch keepAfterTests {
	case "", "never":
		return false
	case "always":
		return true
	case "failure":
		return t.Failed()
	default:
		return false
	}
}

func getDataDirOpt(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--data-dir=") {
			return arg
		}
	}
	return ""
}

func (s *FootlooseSuite) startHAProxy() {
	addresses := s.getControllersIPAddresses()
	ssh, err := s.SSH(s.LBNode())
	s.Require().NoError(err)
	defer ssh.Disconnect()
	content := s.getLBConfig(addresses)

	_, err = ssh.ExecWithOutput(s.Context(), fmt.Sprintf("echo '%s' >%s", content, "/tmp/haproxy.cfg"))

	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput(s.Context(), "haproxy -c -f /tmp/haproxy.cfg")
	s.Require().NoError(err, "LB configuration is broken", err)
	_, err = ssh.ExecWithOutput(s.Context(), "haproxy -D -f /tmp/haproxy.cfg")
	s.Require().NoError(err, "Can't start LB")
}

func (s *FootlooseSuite) getLBConfig(adresses []string) string {
	tpl := `
defaults
    # timeouts are to prevent warning during haproxy -c call
    mode tcp
   	timeout connect 10s
    timeout client 30s
    timeout server 30s

frontend kubeapi

    bind :{{ .KubeAPIExternalPort }}
    default_backend kubeapi

frontend k0sapi
    bind :{{ .K0sAPIExternalPort }}
    default_backend k0sapi

frontend konnectivityAdmin
    bind :{{ .KonnectivityAdminPort }}
    default_backend admin


frontend konnectivityAgent
    bind :{{ .KonnectivityAgentPort }}
    default_backend agent


{{ $OUT := .}}

backend kubeapi
{{ range $addr := .IPAddresses }}
	server  {{ $addr }} {{ $addr }}:{{ $OUT.KubeAPIExternalPort }}
{{ end }}

backend k0sapi
{{ range $addr := .IPAddresses }}
	server {{ $addr }} {{ $addr }}:{{ $OUT.K0sAPIExternalPort }}
{{ end }}

backend admin
{{ range $addr := .IPAddresses }}
	server {{ $addr }} {{ $addr }}:{{ $OUT.KonnectivityAdminPort }}
{{ end }}

backend agent
{{ range $addr := .IPAddresses }}
	server {{ $addr }} {{ $addr }}:{{ $OUT.KonnectivityAgentPort }}
{{ end }}

listen stats
   bind *:9000
   mode http
   stats enable
   stats uri /

`
	content := bytes.NewBuffer([]byte{})
	s.Assert().NoError(template.Must(template.New("haproxy").Parse(tpl)).Execute(content, struct {
		KubeAPIExternalPort   int
		K0sAPIExternalPort    int
		KonnectivityAgentPort int
		KonnectivityAdminPort int

		IPAddresses []string
	}{
		KubeAPIExternalPort:   s.KubeAPIExternalPort,
		K0sAPIExternalPort:    s.K0sAPIExternalPort,
		KonnectivityAdminPort: s.KonnectivityAdminPort,
		KonnectivityAgentPort: s.KonnectivityAgentPort,
		IPAddresses:           adresses,
	}))

	return content.String()
}

func (s *FootlooseSuite) getControllersIPAddresses() []string {
	upstreams := make([]string, s.ControllerCount)
	addresses := make([]string, s.ControllerCount)
	for i := 0; i < s.ControllerCount; i++ {
		upstreams[i] = fmt.Sprintf("controller%d", i)
	}

	machines, err := s.InspectMachines(upstreams)

	s.Require().NoError(err)

	for i := 0; i < s.ControllerCount; i++ {
		// If a network is supplied, the address will need to be obtained from there.
		// Note that this currently uses the first network found.
		if machines[i].Status().IP != "" {
			addresses[i] = machines[i].Status().IP
		} else if len(machines[i].Status().RuntimeNetworks) > 0 {
			addresses[i] = machines[i].Status().RuntimeNetworks[0].IP
		}
	}
	return addresses
}

// InitController initializes a controller
func (s *FootlooseSuite) InitController(idx int, k0sArgs ...string) error {
	controllerNode := s.ControllerNode(idx)
	ssh, err := s.SSH(controllerNode)
	if err != nil {
		return err
	}
	defer ssh.Disconnect()

	if err := s.launchDelegate.InitController(s.Context(), ssh, k0sArgs...); err != nil {
		s.T().Logf("failed to start k0scontroller on %s: %v", controllerNode, err)
		return err
	}

	dataDirOpt := getDataDirOpt(k0sArgs)
	if idx == 0 {
		s.dataDirOpt = dataDirOpt
	}

	return s.WaitForKubeAPI(controllerNode, dataDirOpt)
}

// GetJoinToken generates join token for the asked role
func (s *FootlooseSuite) GetJoinToken(role string, extraArgs ...string) (string, error) {
	// assume we have main on node 0 always
	controllerNode := s.ControllerNode(0)
	s.Contains([]string{"controller", "worker"}, role, "Bad role")
	ssh, err := s.SSH(controllerNode)
	if err != nil {
		return "", err
	}
	defer ssh.Disconnect()

	tokenCmd := fmt.Sprintf("%s token create --role=%s %s 2>/dev/null", s.K0sFullPath, role, strings.Join(extraArgs, " "))
	token, err := ssh.ExecWithOutput(s.Context(), tokenCmd)
	if err != nil {
		return "", fmt.Errorf("can't get join token: %v", err)
	}
	outputParts := strings.Split(token, "\n")
	// in case of no k0s.conf given, there might be warnings on the first few lines

	token = outputParts[len(outputParts)-1]
	return token, nil
}

// RunWorkers joins all the workers to the cluster
func (s *FootlooseSuite) RunWorkers(args ...string) error {
	token, err := s.GetJoinToken("worker", getDataDirOpt(args))
	if err != nil {
		return err
	}
	return s.RunWorkersWithToken(token, args...)
}

func (s *FootlooseSuite) RunWorkersWithToken(token string, args ...string) error {
	for i := 0; i < s.WorkerCount; i++ {
		workerNode := s.WorkerNode(i)
		sshWorker, err := s.SSH(workerNode)
		if err != nil {
			return err
		}
		defer sshWorker.Disconnect()

		if err := s.launchDelegate.InitWorker(s.Context(), sshWorker, token, args...); err != nil {
			s.T().Logf("failed to start k0sworker on %s: %v", workerNode, err)
			return err
		}
	}
	return nil
}

// SSH establishes an SSH connection to the node
func (s *FootlooseSuite) SSH(node string) (*SSHConnection, error) {
	m, err := s.MachineForName(node)
	if err != nil {
		return nil, err
	}

	hostPort, err := m.HostPort(22)
	if err != nil {
		return nil, err
	}

	ssh := &SSHConnection{
		Address: "localhost", // We're always SSH'ing through port mappings
		User:    "root",
		Port:    hostPort,
		KeyPath: s.clusterConfig.Cluster.PrivateKey,
	}

	err = ssh.Connect(s.Context())
	if err != nil {
		return nil, err
	}

	return ssh, nil
}

func (s *FootlooseSuite) InspectMachines(hostnames []string) ([]*cluster.Machine, error) {
	return s.cluster.Inspect(hostnames)
}

// MachineForName gets the named machine details
func (s *FootlooseSuite) MachineForName(name string) (*cluster.Machine, error) {
	machines, err := s.InspectMachines(nil)
	if err != nil {
		return nil, err
	}
	for _, m := range machines {
		if m.Hostname() == name {
			return m, nil
		}
	}

	return nil, fmt.Errorf("no machine found with name %s", name)
}

func (s *FootlooseSuite) StopController(name string) error {
	ssh, err := s.SSH(name)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	s.T().Log("killing k0s")

	return s.launchDelegate.StopController(s.Context(), ssh)
}

func (s *FootlooseSuite) Reset(name string) error {
	ssh, err := s.SSH(name)
	s.Require().NoError(err)
	defer ssh.Disconnect()
	resetCommand := fmt.Sprintf("%s reset --debug", s.K0sFullPath)
	_, err = ssh.ExecWithOutput(s.Context(), resetCommand)
	return err
}

// KubeClient return kube client by loading the admin access config from given node
func (s *FootlooseSuite) GetKubeConfig(node string, k0sKubeconfigArgs ...string) (*rest.Config, error) {
	machine, err := s.MachineForName(node)
	if err != nil {
		return nil, err
	}
	ssh, err := s.SSH(node)
	if err != nil {
		return nil, err
	}
	defer ssh.Disconnect()

	kubeConfigCmd := fmt.Sprintf("%s kubeconfig admin %s 2>/dev/null", s.K0sFullPath, strings.Join(k0sKubeconfigArgs, " "))
	kubeConf, err := ssh.ExecWithOutput(s.Context(), kubeConfigCmd)
	if err != nil {
		return nil, err
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConf))
	s.Require().NoError(err)

	hostURL, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("can't parse port value `%s`: %w", cfg.Host, err)
	}
	port, err := strconv.ParseInt(hostURL.Port(), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("can't parse port value `%s`: %w", hostURL.Port(), err)
	}
	hostPort, err := machine.HostPort(int(port))
	if err != nil {
		return nil, fmt.Errorf("footloose machine has to have %d port mapped: %w", port, err)
	}
	cfg.Host = fmt.Sprintf("localhost:%d", hostPort)
	return cfg, nil
}

// CreateUserAndGetKubeClientConfig creates user and returns the kubeconfig as clientcmdapi.Config struct so it can be
// used and loaded with clientsets directly
func (s *FootlooseSuite) CreateUserAndGetKubeClientConfig(node string, username string, k0sKubeconfigArgs ...string) (*rest.Config, error) {
	machine, err := s.MachineForName(node)
	if err != nil {
		return nil, err
	}
	ssh, err := s.SSH(node)
	if err != nil {
		return nil, err
	}
	defer ssh.Disconnect()

	kubeConfigCmd := fmt.Sprintf("%s kubeconfig create %s %s 2>/dev/null", s.K0sFullPath, username, strings.Join(k0sKubeconfigArgs, " "))
	kubeConf, err := ssh.ExecWithOutput(s.Context(), kubeConfigCmd)
	if err != nil {
		return nil, err
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConf))
	s.Require().NoError(err)

	hostURL, err := url.Parse(cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("can't parse port value `%s`: %w", cfg.Host, err)
	}
	port, err := strconv.ParseInt(hostURL.Port(), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("can't parse port value `%s`: %w", hostURL.Port(), err)
	}
	hostPort, err := machine.HostPort(int(port))
	if err != nil {
		return nil, fmt.Errorf("footloose machine has to have %d port mapped: %w", port, err)
	}
	cfg.Host = fmt.Sprintf("localhost:%d", hostPort)
	return cfg, nil
}

// KubeClient return kube client by loading the admin access config from given node
func (s *FootlooseSuite) KubeClient(node string, k0sKubeconfigArgs ...string) (*kubernetes.Clientset, error) {
	cfg, err := s.GetKubeConfig(node, k0sKubeconfigArgs...)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

// AutopilotClient returns a client for accessing the autopilot schema
func (s *FootlooseSuite) AutopilotClient(node string, k0sKubeconfigArgs ...string) (apclient.Interface, error) {
	cfg, err := s.GetKubeConfig(node, k0sKubeconfigArgs...)
	if err != nil {
		return nil, err
	}
	return apclient.NewForConfig(cfg)
}

// ExtensionsClient returns a client for accessing the extensions schema
func (s *FootlooseSuite) ExtensionsClient(node string, k0sKubeconfigArgs ...string) (*extclient.ApiextensionsV1Client, error) {
	cfg, err := s.GetKubeConfig(node, k0sKubeconfigArgs...)
	if err != nil {
		return nil, err
	}

	return extclient.NewForConfig(cfg)
}

// WaitForNodeReady wait that we see the given node in "Ready" state in kubernetes API
func (s *FootlooseSuite) WaitForNodeReady(name string, kc *kubernetes.Clientset) error {
	s.T().Logf("waiting to see %s ready in kube API", name)
	return watch.Nodes(kc.CoreV1().Nodes()).
		WithObjectName(name).
		Until(s.Context(), func(n *corev1.Node) (bool, error) {
			for _, nc := range n.Status.Conditions {
				if nc.Type == corev1.NodeReady {
					if nc.Status == corev1.ConditionTrue {
						s.T().Logf("%s is ready in API", n.Name)
						return true, nil
					}

					break
				}
			}

			return false, nil
		})
}

// GetNodeLabels return the labels of given node
func (s *FootlooseSuite) GetNodeLabels(node string, kc *kubernetes.Clientset) (map[string]string, error) {
	n, err := kc.CoreV1().Nodes().Get(s.Context(), node, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return n.Labels, nil
}

// WaitForNodeLabel waits for label be assigned to the node
func (s *FootlooseSuite) WaitForNodeLabel(kc *kubernetes.Clientset, node, labelKey, labelValue string) error {
	return watch.Nodes(kc.CoreV1().Nodes()).
		WithObjectName(node).
		Until(s.Context(), func(node *corev1.Node) (bool, error) {
			for k, v := range node.Labels {
				if labelKey == k {
					if labelValue == v {
						return true, nil
					}

					break
				}
			}

			return false, nil
		})
}

// GetNodeLabels return the labels of given node
func (s *FootlooseSuite) GetNodeAnnotations(node string, kc *kubernetes.Clientset) (map[string]string, error) {
	n, err := kc.CoreV1().Nodes().Get(s.Context(), node, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return n.Annotations, nil
}

// AddNodeLabel adds a label to the provided node.
func (s *FootlooseSuite) AddNodeLabel(node string, kc *kubernetes.Clientset, key string, value string) (*corev1.Node, error) {
	return nodeValuePatchAdd(s.Context(), node, kc, "/metadata/labels", key, value)
}

// AddNodeAnnotation adds an annotation to the provided node.
func (s *FootlooseSuite) AddNodeAnnotation(node string, kc *kubernetes.Clientset, key string, value string) (*corev1.Node, error) {
	return nodeValuePatchAdd(s.Context(), node, kc, "/metadata/annotations", key, value)
}

// nodeValuePatchAdd patch-adds a key/value to a specific path via the Node API
func nodeValuePatchAdd(ctx context.Context, node string, kc *kubernetes.Clientset, path string, key string, value string) (*corev1.Node, error) {
	keyPath := fmt.Sprintf("%s/%s", path, jsonpointer.Escape(key))
	patch := fmt.Sprintf(`[{"op":"add", "path":"%s", "value":"%s" }]`, keyPath, value)
	return kc.CoreV1().Nodes().Patch(ctx, node, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
}

// WaitForKubeAPI waits until we see kube API online on given node.
// Timeouts with error return in 5 mins
func (s *FootlooseSuite) WaitForKubeAPI(node string, k0sKubeconfigArgs ...string) error {
	s.T().Logf("waiting for kube api to start on node %s", node)
	return Poll(s.Context(), func(context.Context) (done bool, err error) {
		kc, err := s.KubeClient(node, k0sKubeconfigArgs...)
		if err != nil {
			s.T().Logf("kube-client error: %v", err)
			return false, nil
		}
		v, err := kc.ServerVersion()
		if err != nil {
			s.T().Logf("server version error: %v", err)
			return false, nil
		}
		ctx, cancel := context.WithTimeout(s.Context(), 5*time.Second)
		defer cancel()
		res := kc.RESTClient().Get().RequestURI("/readyz").Do(ctx)
		if res.Error() != nil {
			return false, nil
		}
		var statusCode int
		res.StatusCode(&statusCode)
		if statusCode != http.StatusOK {
			s.T().Logf("status not ok. code: %v", statusCode)
			return false, nil
		}

		s.T().Logf("kube api up-and-running, version: %s", v.String())

		return true, nil
	})
}

// WaitJoinApi waits until we see k0s join api up-and-running on a given node
// Timeouts with error return in 5 mins
func (s *FootlooseSuite) WaitJoinAPI(node string) error {
	s.T().Logf("waiting for join api to start on node %s", node)
	return Poll(s.Context(), func(context.Context) (done bool, err error) {
		joinAPIStatus, err := s.GetHTTPStatus(node, "/v1beta1/ca")
		if err != nil {
			return false, nil
		}
		// JoinAPI returns always un-authorized when called with no token, but it's a signal that it properly up-and-running still
		if joinAPIStatus != http.StatusUnauthorized {
			return false, nil
		}

		s.T().Logf("join api up-and-running")

		return true, nil
	})
}

func (s *FootlooseSuite) GetHTTPStatus(node string, path string) (int, error) {
	m, err := s.MachineForName(node)
	if err != nil {
		return 0, err
	}
	joinPort, err := m.HostPort(s.K0sAPIExternalPort)
	if err != nil {
		return 0, err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	checkURL := fmt.Sprintf("https://localhost:%d/%s", joinPort, path)
	resp, err := client.Get(checkURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func (s *FootlooseSuite) initializeFootlooseCluster() error {
	dir, err := os.MkdirTemp("", s.T().Name()+"-footloose.")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory for footloose configuration: %w", err)
	}

	err = s.initializeFootlooseClusterInDir(dir)
	if err != nil {
		cleanupClusterDir(s.T(), dir)
	}

	return err
}

// Verifies that kubelet process has the address flag set
func (s *FootlooseSuite) GetKubeletCMDLine(node string) (string, error) {
	ssh, err := s.SSH(node)
	if err != nil {
		return "", err
	}
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(s.Context(), `cat /proc/$(pidof kubelet)/cmdline`)
	if err != nil {
		return "", err
	}

	return output, nil
}

func (s *FootlooseSuite) initializeFootlooseClusterInDir(dir string) error {
	binPath := os.Getenv("K0S_PATH")
	if binPath == "" {
		return errors.New("failed to locate k0s binary: K0S_PATH environment variable not set")
	}

	fileInfo, err := os.Stat(binPath)
	if err != nil {
		return fmt.Errorf("failed to locate k0s binary %s: %w", binPath, err)
	}
	if fileInfo.IsDir() {
		return fmt.Errorf("failed to locate k0s binary %s: is a directory", binPath)
	}

	volumes := []config.Volume{
		{
			Type:        "volume",
			Destination: "/var/lib/k0s",
		},
	}

	updateFromBinPath := os.Getenv("K0S_UPDATE_FROM_PATH")
	if updateFromBinPath != "" {
		volumes = append(volumes, config.Volume{
			Type:        "bind",
			Source:      updateFromBinPath,
			Destination: k0sBindMountFullPath,
			ReadOnly:    true,
		}, config.Volume{
			Type:        "bind",
			Source:      binPath,
			Destination: k0sNewBindMountFullPath,
			ReadOnly:    true,
		})
	} else {
		volumes = append(volumes, config.Volume{
			Type:        "bind",
			Source:      binPath,
			Destination: k0sBindMountFullPath,
			ReadOnly:    true,
		})
	}

	if len(s.AirgapImageBundleMountPoints) > 0 {
		airgapPath, ok := os.LookupEnv("K0S_IMAGES_BUNDLE")
		if !ok {
			return errors.New("cannot bind-mount airgap image bundle, environment variable K0S_IMAGES_BUNDLE not set")
		} else if !file.Exists(airgapPath) {
			return fmt.Errorf("cannot bind-mount airgap image bundle, no such file: %q", airgapPath)
		}

		for _, dest := range s.AirgapImageBundleMountPoints {
			volumes = append(volumes, config.Volume{
				Type:        "bind",
				Source:      airgapPath,
				Destination: dest,
				ReadOnly:    true,
			})
		}
	}

	// Ensure that kernel config is available in the footloose boxes.
	// See https://github.com/kubernetes/system-validators/blob/v1.6.0/validators/kernel_validator.go#L180-L190

	bindPaths := []string{
		"/usr/src/linux/.config",
		"/usr/lib/modules",
		"/lib/modules",
	}

	if kernelVersion, err := exec.Command("uname", "-r").Output(); err == nil {
		kernelVersion := strings.TrimSpace(string(kernelVersion))
		bindPaths = append(bindPaths, []string{
			"/boot/config-" + kernelVersion,
			"/usr/src/linux-" + kernelVersion,
			"/usr/lib/ostree-boot/config-" + kernelVersion,
			"/usr/lib/kernel/config-" + kernelVersion,
			"/usr/src/linux-headers-" + kernelVersion,
		}...)
	} else {
		s.T().Logf("not mounting any kernel-specific paths: %v", err)
	}

	for _, path := range bindPaths {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		volumes = append(volumes, config.Volume{
			Type:        "bind",
			Source:      path,
			Destination: path,
			ReadOnly:    true,
		})
	}

	volumes = append(volumes, s.ExtraVolumes...)

	s.T().Logf("mounting volumes: %v", volumes)

	portMaps := []config.PortMapping{
		{
			ContainerPort: 22, // SSH
		},
		{
			ContainerPort: 10250, // kubelet logs
		},
		{
			ContainerPort: uint16(s.K0sAPIExternalPort), // kube API
		},
		{
			ContainerPort: uint16(s.KubeAPIExternalPort), // kube API
		},
		{
			ContainerPort: uint16(6060), // pprof API
		},
	}

	cfg := config.Config{
		Cluster: config.Cluster{
			Name:       s.T().Name(),
			PrivateKey: path.Join(dir, "id_rsa"),
		},
		Machines: []config.MachineReplicas{
			{
				Count: s.ControllerCount,
				Spec: config.Machine{
					Image:        "footloose-alpine",
					Name:         controllerNodeNameFormat,
					Privileged:   true,
					Volumes:      volumes,
					PortMappings: portMaps,
					Networks:     s.ControllerNetworks,
				},
			},
			{
				Count: s.WorkerCount,
				Spec: config.Machine{
					Image:        "footloose-alpine",
					Name:         workerNodeNameFormat,
					Privileged:   true,
					Volumes:      volumes,
					PortMappings: portMaps,
					Networks:     s.WorkerNetworks,
				},
			},
		},
	}

	if s.WithLB {
		cfg.Machines = append(cfg.Machines, config.MachineReplicas{
			Spec: config.Machine{
				Name:         lbNodeNameFormat,
				Image:        "footloose-alpine",
				Privileged:   true,
				Volumes:      volumes,
				PortMappings: portMaps,
				Ignite:       nil,
				Networks:     s.ControllerNetworks,
			},
			Count: 1,
		})
	}

	if s.WithExternalEtcd {
		cfg.Machines = append(cfg.Machines, config.MachineReplicas{
			Spec: config.Machine{
				Name:         etcdNodeNameFormat,
				Image:        "footloose-alpine",
				Privileged:   true,
				PortMappings: []config.PortMapping{{ContainerPort: 22}},
			},
			Count: 1,
		})
	}

	if s.WithUpdateServer {
		cfg.Machines = append(cfg.Machines, config.MachineReplicas{
			Spec: config.Machine{
				Name:       updateServerNodeNameFormat,
				Image:      "update-server",
				Privileged: true,
				PortMappings: []config.PortMapping{
					{
						ContainerPort: 22, // SSH
					},
					{
						ContainerPort: 80,
					},
				},
				Networks: s.ControllerNetworks,
			},
			Count: 1,
		})
	}

	footlooseYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal footloose configuration: %w", err)
	}

	if err = os.WriteFile(path.Join(dir, "footloose.yaml"), footlooseYaml, 0700); err != nil {
		return fmt.Errorf("failed to write footloose configuration to file: %w", err)
	}

	cluster, err := cluster.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to setup a new footloose cluster: %w", err)
	}

	// we first try to delete instances from previous runs, if they happen to exist
	_ = cluster.Delete()
	if err := cluster.Create(); err != nil {
		return fmt.Errorf("failed to create footloose cluster: %w", err)
	}

	s.clusterDir = dir
	s.clusterConfig = cfg
	s.cluster = cluster
	return nil
}

func cleanupClusterDir(t *testing.T, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("failed to remove footloose configuration directory %s: %v", dir, err)
	}
}

func newSuiteContext(t *testing.T) (context.Context, context.CancelFunc) {
	// We need to reserve some time to conduct a proper teardown of the suite before the test timeout kicks in.
	if deadline, hasDeadline := t.Deadline(); hasDeadline {
		remainingTestDuration := time.Until(deadline)
		//  Let's reserve 10% ...
		reservedTeardownDuration := time.Duration(float64(remainingTestDuration.Milliseconds())*0.10) * time.Millisecond
		// ... but at least 20 seconds.
		reservedTeardownDuration = time.Duration(math.Max(float64(20*time.Second), float64(reservedTeardownDuration)))
		// And construct the context accordingly
		return context.WithDeadline(context.Background(), deadline.Add(-reservedTeardownDuration))
	}

	return context.WithCancel(context.Background())
}

// GetControllerIPAddress returns controller ip address
func (s *FootlooseSuite) GetControllerIPAddress(idx int) string {
	return s.getIPAddress(s.ControllerNode(idx))
}

func (s *FootlooseSuite) GetWorkerIPAddress(idx int) string {
	return s.getIPAddress(s.WorkerNode(idx))
}

func (s *FootlooseSuite) GetLBAddress() string {
	return s.getIPAddress(s.LBNode())
}

func (s *FootlooseSuite) GetExternalEtcdIPAddress() string {
	return s.getIPAddress(s.ExternalEtcdNode())
}

func (s *FootlooseSuite) getIPAddress(nodeName string) string {
	ssh, err := s.SSH(nodeName)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	ipAddress, err := ssh.ExecWithOutput(s.Context(), "hostname -i")
	s.Require().NoError(err)
	return ipAddress
}

// CreateNetwork creates a docker network with the provided name, destroying
// any network that has the same name first.
func (s *FootlooseSuite) CreateNetwork(name string) error {
	_ = s.DestroyNetwork(name)

	cmd := exec.Command("docker", "network", "create", name)
	return cmd.Run()
}

// DestroyNetwork removes a docker network with the provided name.
func (s *FootlooseSuite) DestroyNetwork(name string) error {
	cmd := exec.Command("docker", "network", "rm", name)
	return cmd.Run()
}

// RunCommandController runs a command via SSH on a specified controller node
func (s *FootlooseSuite) RunCommandController(idx int, command string) (string, error) {
	ssh, err := s.SSH(s.ControllerNode(idx))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	return ssh.ExecWithOutput(s.Context(), command)
}

// RunCommandWorker runs a command via SSH on a specified controller node
func (s *FootlooseSuite) RunCommandWorker(idx int, command string) (string, error) {
	ssh, err := s.SSH(s.WorkerNode(idx))
	s.Require().NoError(err)
	defer ssh.Disconnect()

	return ssh.ExecWithOutput(s.Context(), command)
}

// GetK0sVersion returns the `k0s version` output from a specific node.
func (s *FootlooseSuite) GetK0sVersion(node string) (string, error) {
	ssh, err := s.SSH(node)
	if err != nil {
		return "", err
	}
	defer ssh.Disconnect()

	version, err := ssh.ExecWithOutput(s.Context(), "/usr/local/bin/k0s version")
	if err != nil {
		return "", err
	}

	return version, nil
}

// GetMembers returns all of the known etcd members for a given node
func (s *FootlooseSuite) GetMembers(idx int) map[string]string {
	// our etcd instances doesn't listen on public IP, so test is performed by calling CLI tools over ssh
	// which in general even makes sense, we can test tooling as well
	sshCon, err := s.SSH(s.ControllerNode(idx))
	s.Require().NoError(err)
	defer sshCon.Disconnect()
	output, err := sshCon.ExecWithOutput(s.Context(), "/usr/local/bin/k0s etcd member-list")
	s.Require().NoError(err)
	output = lastLine(output)

	members := struct {
		Members map[string]string `json:"members"`
	}{}

	s.Require().NoError(json.Unmarshal([]byte(output), &members))

	return members.Members
}

func lastLine(text string) string {
	if text == "" {
		return ""
	}
	parts := strings.Split(text, "\n")
	return parts[len(parts)-1]
}

// WaitForSSH ensures that an SSH connection can be successfully obtained, and retries
// for up to a specific timeout/delay.
func (s *FootlooseSuite) WaitForSSH(node string, timeout time.Duration, delay time.Duration) error {
	s.T().Logf("Waiting for SSH connection to '%s'", node)
	for start := time.Now(); time.Since(start) < timeout; {
		if conn, err := s.SSH(node); err == nil {
			conn.Disconnect()
			return nil
		}

		s.T().Logf("Unable to SSH to '%s', waiting %v for retry", node, delay)
		time.Sleep(delay)
	}

	return fmt.Errorf("timed out waiting for ssh connection to '%s'", node)
}

// GetUpdateServerIPAddress returns the load balancers ip address
func (s *FootlooseSuite) GetUpdateServerIPAddress() string {
	ssh, err := s.SSH("updateserver0")
	s.Require().NoError(err)
	defer ssh.Disconnect()

	ipAddress, err := ssh.ExecWithOutput(s.Context(), "hostname -i")
	s.Require().NoError(err)
	return ipAddress
}

func (s *FootlooseSuite) AssertSomeKubeSystemPods(client *kubernetes.Clientset) bool {
	if pods, err := client.CoreV1().Pods("kube-system").List(s.Context(), v1.ListOptions{
		Limit: 100,
	}); s.NoError(err) {
		s.T().Logf("Found %d pods in kube-system", len(pods.Items))
		return s.NotEmpty(pods.Items, "Expected to see some pods in kube-system namespace")
	}

	return false
}
