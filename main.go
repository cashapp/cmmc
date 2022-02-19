/*
Copyright 2021 Square, Inc

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

package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	cmmcv1beta1 "github.com/cashapp/cmmc/api/v1beta1"
	"github.com/cashapp/cmmc/controllers"
	"github.com/cashapp/cmmc/util/metrics"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(cmmcv1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() { //nolint:funlen
	var (
		metricsAddr                        string
		enableLeaderElection               bool
		probeAddr                          string
		mergeTargetMaxConcurrentReconciles int
		mergeSourceMaxConcurrentReconciles int
		displayHelp                        bool
		opts                               = zap.Options{Development: true}
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.IntVar(&mergeTargetMaxConcurrentReconciles, "merge-target-max-concurrent-reconciles", 1, "MergeTargetController - MaxConcurrentReconciles")
	flag.IntVar(&mergeSourceMaxConcurrentReconciles, "merge-source-max-concurrent-reconciles", 1, "MergeSourceController - MaxConcurrentReconciles")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&displayHelp, "help", false, "Display usage")
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if displayHelp {
		flag.Usage()
		os.Exit(0)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	recorder := initRecorder()
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443, //nolint: gomnd
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "6952adb5.cmmc.k8s.cash.app",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.MergeSourceReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
	}).SetupWithManager(mgr, controller.Options{
		MaxConcurrentReconciles: mergeSourceMaxConcurrentReconciles,
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MergeSource")
		os.Exit(1)
	}

	if err = (&controllers.MergeTargetReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: recorder,
	}).SetupWithManager(mgr, controller.Options{
		MaxConcurrentReconciles: mergeSourceMaxConcurrentReconciles,
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MergeTarget")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func initRecorder() *metrics.Recorder {
	recorder := metrics.NewRecorder()
	ctrlmetrics.Registry.MustRegister(recorder.Collectors()...)
	return recorder
}
