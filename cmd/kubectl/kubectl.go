/*
Copyright 2021 k0s authors

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
package kubectl

import (
	"os"

	"github.com/k0sproject/k0s/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/component-base/cli"
	kubectl "k8s.io/kubectl/pkg/cmd"
	"k8s.io/kubectl/pkg/cmd/plugin"
)

type CmdOpts config.CLIOptions

func NewK0sKubectlCmd() *cobra.Command {
	_ = pflag.CommandLine.MarkHidden("log-flush-frequency")
	_ = pflag.CommandLine.MarkHidden("version")

	wrapperCmd := &cobra.Command{
		Use: "kubectl",
		Run: func(argCmd *cobra.Command, args []string) {
			kubectlCmd := kubectl.NewDefaultKubectlCommandWithArgs(kubectl.KubectlOptions{
				PluginHandler: kubectl.NewDefaultPluginHandler(plugin.ValidPluginFilenamePrefixes),
				Arguments:     args,
				ConfigFlags:   genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag(),
				IOStreams:     genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
			})
			// spew.Dump(args)
			// spew.Dump(os.Args)
			os.Args = os.Args[1:]
			os.Args[0] = "kubectl"
			// workaround for the data-dir location input for the kubectl command
			kubectlCmd.Aliases = []string{"kc"}
			// Get handle on the original kubectl prerun so we can call it later
			originalPreRunE := kubectlCmd.PersistentPreRunE
			kubectlCmd.PersistentPreRunE = func(prerunCmd *cobra.Command, args []string) error {
				c := CmdOpts(config.GetCmdOpts())
				if os.Getenv("KUBECONFIG") == "" {
					// Verify we can read the config before pushing it to env
					file, err := os.OpenFile(c.K0sVars.AdminKubeConfigPath, os.O_RDONLY, 0600)
					if err != nil {
						logrus.Errorf("cannot read admin kubeconfig at %s, is the server running?", c.K0sVars.AdminKubeConfigPath)
						return err
					}
					defer file.Close()
					os.Setenv("KUBECONFIG", c.K0sVars.AdminKubeConfigPath)
				}

				return originalPreRunE(prerunCmd, args)
			}

			cli.Run(kubectlCmd)
		},
	}
	wrapperCmd.Aliases = []string{"kc"}
	return wrapperCmd
}
