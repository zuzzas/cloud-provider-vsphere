/*
Copyright 2018 The Kubernetes Authors.

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

// The external controller manager is responsible for running controller loops that
// are cloud provider dependent. It uses the API to listen to new events on resources.

package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/pkg/util/logs"
	"k8s.io/kubernetes/cmd/cloud-controller-manager/app"
	"k8s.io/kubernetes/cmd/cloud-controller-manager/app/options"
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus" // for client metric registration
	utilflag "k8s.io/kubernetes/pkg/util/flag"
	_ "k8s.io/kubernetes/pkg/version/prometheus" // for version metric registration
	"k8s.io/kubernetes/pkg/version/verflag"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog"
)

var version string

func init() {
	healthz.DefaultHealthz()
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	goflag.Set("logtostderr", "true")
	goflag.Set("stderrthreshold", "INFO")
	goflag.Set("alsologtostderr", "true")
	goflag.CommandLine.Parse([]string{})
	s, err := options.NewCloudControllerManagerOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	command := &cobra.Command{
		Use: "cloud-controller-manager",
		Long: `The Cloud controller manager is a daemon that embeds
the cloud specific control loops shipped with Kubernetes.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Glog requires this otherwise it complains.
			goflag.CommandLine.Parse(nil)

			// TODO: We need to revisit this when we vendor newer components of k8s
			// TODO: such as but not limited to k/k, client, apimachinery, etc
			//
			// This is a temporary hack to enable proper logging until upstream dependencies
			// are migrated to fully utilize klog instead of glog.
			klogFlags := goflag.NewFlagSet("klog", goflag.ExitOnError)
			klog.InitFlags(klogFlags)

			// Sync the glog and klog flags.
			cmd.Flags().VisitAll(func(f1 *pflag.Flag) {
				f2 := klogFlags.Lookup(f1.Name)
				if f2 != nil {
					value := f1.Value.String()
					f2.Value.Set(value)
				}
			})
		},
		Run: func(cmd *cobra.Command, args []string) {
			verflag.PrintAndExitIfRequested()
			utilflag.PrintFlags(cmd.Flags())

			c, err := s.Config()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			if err := app.Run(c.Complete()); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

		},
	}
	s.AddFlags(command.Flags())

	// TODO: once we switch everything over to Cobra commands, we can go back to calling
	// utilflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
	// normalize func and add the go flag set by hand.
	pflag.CommandLine.SetNormalizeFunc(flag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	// utilflag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	klog.V(1).Infof("vsphere-cloud-controller-manager version: %s", version)

	s.CloudProvider.Name = vsphere.ProviderName
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
