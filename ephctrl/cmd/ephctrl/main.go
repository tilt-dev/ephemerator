package main

import (
	"os"
	"time"

	"github.com/tilt-dev/ephemerator/ephconfig"
	"github.com/tilt-dev/ephemerator/ephctrl/pkg/env"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func init() {
	log.SetLogger(zap.New())
}

func main() {
	l := log.Log.WithName("ephctrl")
	s := runtime.NewScheme()
	err := scheme.AddToScheme(s)
	if err != nil {
		l.Error(err, "scheme setup failed")
		os.Exit(1)
	}

	timeout := 15 * time.Second
	mgr, err := ctrl.NewManager(config.GetConfigOrDie(), ctrl.Options{
		Scheme: s,
		Port:   9443,

		// leader election is unnecessary as we run this as a single manager process.
		LeaderElection:   false,
		LeaderElectionID: "ephctrl",

		Logger:                  l,
		GracefulShutdownTimeout: &timeout,
	})
	if err != nil {
		l.Error(err, "manager setup failed")
		os.Exit(1)
	}

	allowlist, err := ephconfig.ReadAllowlist()
	if err != nil {
		l.Error(err, "controller setup failed")
		os.Exit(1)
	}

	r, err := env.NewReconciler(mgr, allowlist)
	if err != nil {
		l.Error(err, "controller setup failed")
		os.Exit(1)
	}

	err = r.AddToManager(mgr)
	if err != nil {
		l.Error(err, "controller setup failed")
		os.Exit(1)
	}

	gr := env.NewGatewayReconciler(mgr)
	err = gr.AddToManager(mgr)
	if err != nil {
		l.Error(err, "controller setup failed")
		os.Exit(1)
	}

	l.Info("starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		l.Error(err, "manager start failed")
		os.Exit(1)
	}
}
