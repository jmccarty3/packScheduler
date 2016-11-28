package main

import (
	"flag"
	"runtime"

	_ "github.com/jmccarty3/packScheduler/algorithm"

	"k8s.io/kubernetes/pkg/healthz"
	k8sFlag "k8s.io/kubernetes/pkg/util/flag"
	"k8s.io/kubernetes/pkg/util/logs"
	"k8s.io/kubernetes/pkg/version/verflag"
	"k8s.io/kubernetes/plugin/cmd/kube-scheduler/app"
	"k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"

	"github.com/spf13/pflag"
)

func init() {
	healthz.DefaultHealthz()
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	s := options.NewSchedulerServer()
	s.AddFlags(pflag.CommandLine)

	k8sFlag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	verflag.PrintAndExitIfRequested()
	// Trick to avoid 'logging before flag.Parse' warning
	flag.CommandLine.Parse([]string{})
	app.Run(s)
}
