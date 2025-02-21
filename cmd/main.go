/*
Copyright 2025.

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
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"

	// Import all Kubernetes client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	apiv1alpha1 "github.com/oleksandrfrolov95/debezium-operator/api/v1alpha1"
	"github.com/oleksandrfrolov95/debezium-operator/internal/controller"
	"github.com/oleksandrfrolov95/debezium-operator/internal/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrllog.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Directory where cert files will be stored.
	const certDir = "/tmp/certs"
	if err := os.MkdirAll(certDir, 0755); err != nil {
		fmt.Printf("failed to create cert directory %s: %v\n", certDir, err)
		os.Exit(1)
	}

	// Get the webhook service name and namespace from environment variables.
	serviceName := os.Getenv("WEBHOOK_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "debezium-operator"
	}
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = "debezium-operator-ns"
	}
	// Build the common name.
	commonName := fmt.Sprintf("%s.%s.svc", serviceName, namespace)
	fmt.Printf("Using commonName: %s\n", commonName)

	// Setup TLS options: disable HTTP/2 if not enabled.
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}
	var tlsOpts []func(*tls.Config)
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Create the webhook server with the specified certificate directory.
	webhookServer := webhook.NewServer(webhook.Options{
		Port:    8443,
		TLSOpts: tlsOpts,
		CertDir: certDir,
	})

	// Create the manager using ctrl.NewManager.
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "7b7a467c.debezium",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Create a direct (non-cached) client for certificate bootstrapping.
	cfg := ctrl.GetConfigOrDie()
	directClient, err := client.New(cfg, client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		setupLog.Error(err, "unable to create direct client")
		os.Exit(1)
	}

	// Use the direct client to load or generate the certificate.
	const secretName = "debezium-operator-tls"
	ctx := context.Background()
	if err := util.LoadOrGenerateCert(ctx, directClient, namespace, secretName, certDir, commonName); err != nil {
		setupLog.Error(err, "failed to load or generate certificate")
		os.Exit(1)
	}

	// Update the ValidatingWebhookConfiguration with the CA bundle from the TLS secret.
	// This logic is now in the util package.
	const webhookName = "vdebeziumconnector.api.debezium.io"
	const vwcName = "debeziumconnectors-validating-webhook"
	if err := util.UpdateWebhookCABundle(ctx, directClient, webhookName, vwcName, namespace, secretName); err != nil {
		setupLog.Error(err, "failed to update webhook caBundle")
		os.Exit(1)
	}

	// Setup controllers.
	if err = (&controller.DebeziumConnectorReconciler{
		Client:     mgr.GetClient(),
		HTTPClient: mgr.GetHTTPClient(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DebeziumConnector")
		os.Exit(1)
	}

	// Register the webhook for DebeziumConnector.
	if err := (&apiv1alpha1.DebeziumConnector{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "DebeziumConnector")
		os.Exit(1)
	}

	// Add health and ready checks.
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
