package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	//+kubebuilder:scaffold:imports

	ing "kuberstein.io/ingressh/api/v1"
	"kuberstein.io/ingressh/internal/controller"
	"kuberstein.io/ingressh/internal/k8s"
	"kuberstein.io/ingressh/internal/server"
	"kuberstein.io/ingressh/internal/types"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(ing.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var sshConfig string
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&sshConfig, "ssh-config", "", "Path to the configuration file for the SSH server.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "af6811ad.kuberstein.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.IngreSshReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IngreSsh")
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

	ctx := ctrl.SetupSignalHandler()
	eg, egCtx := errgroup.WithContext(ctx)

	setupLog.Info("Starting controller manager...")
	eg.Go(func() error {
		if err := mgr.Start(egCtx); err != nil {
			return fmt.Errorf("problem running controller manager: %v", err)
		}
		return nil
	})

	setupLog.Info("Starting SSH server...")
	eg.Go(func() error {
		return startSshServer(sshConfig, egCtx)
	})

	if err := eg.Wait(); err != nil {
		setupLog.Error(err, "problem starting services")
	}
}

func startSshServer(sshConfigPath string, ctx context.Context) error {

	conf, err := types.GetServerConf(sshConfigPath)
	if err != nil {
		return err
	}

	kube := k8s.ClientImpl{}
	if err := kube.Init(ctrl.GetConfigOrDie()); err != nil {
		return fmt.Errorf("unable to create K8s client: %v", err)
	}

	ln, err := net.Listen("tcp", conf.BindAddress)
	if err != nil {
		return fmt.Errorf("unable to listen socket at %s: %v", conf.BindAddress, err)
	}

	pemBytes, err := os.ReadFile(conf.HostKeyFile)
	if err != nil {
		return fmt.Errorf("unable to read host key file %s: %v", conf.HostKeyFile, err)
	}

	signer, err := gossh.ParsePrivateKey(pemBytes)
	if err != nil {
		return fmt.Errorf("unable to parse private key: %v", err)
	}

	srv := &ssh.Server{
		PublicKeyHandler: server.PublicKeyAuthHandler,
		Handler:          server.GetHandler(&kube, conf),
		HostSigners:      []ssh.Signer{signer},
	}

	setupLog.Info("Starting ssh ingress server", "address", conf.BindAddress)

	go srv.Serve(ln)
	for {
		select {
		case <-ctx.Done():
			return srv.Close()
		}
	}
}
