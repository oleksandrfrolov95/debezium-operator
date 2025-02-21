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

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1alpha1 "github.com/oleksandrfrolov95/debezium-operator/api/v1alpha1"
	"github.com/oleksandrfrolov95/debezium-operator/internal/util"
)

// DebeziumConnectorReconciler reconciles a DebeziumConnector object
type DebeziumConnectorReconciler struct {
	client.Client
	HTTPClient *http.Client
}

// Finalizer name for DebeziumConnector
const debeziumFinalizer = "debeziumconnector.finalizers.api.debezium"

//+kubebuilder:rbac:groups=api.debezium,resources=debeziumconnectors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=api.debezium,resources=debeziumconnectors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=api.debezium,resources=debeziumconnectors/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;update;patch

func (r *DebeziumConnectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	dbc := &apiv1alpha1.DebeziumConnector{}
	if err := r.Get(ctx, req.NamespacedName, dbc); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("DebeziumConnector resource not found; it may have been deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get DebeziumConnector")
		return ctrl.Result{}, err
	}

	// Initialize HTTP client if not already set.
	if r.HTTPClient == nil {
		r.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}

	// Handle deletion: If the resource is being deleted, remove the connector from Debezium.
	if !dbc.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(dbc, debeziumFinalizer) {
			if err := r.deleteDebeziumConnector(dbc.Spec.DebeziumHost, dbc.Spec.Config["name"]); err != nil {
				logger.Error(err, "failed to delete Debezium connector")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(dbc, debeziumFinalizer)
			if err := r.Update(ctx, dbc); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure our finalizer is present.
	if !controllerutil.ContainsFinalizer(dbc, debeziumFinalizer) {
		controllerutil.AddFinalizer(dbc, debeziumFinalizer)
		if err := r.Update(ctx, dbc); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check if the connector already exists on the Debezium host.
	exists, err := r.connectorExists(dbc.Spec.DebeziumHost, dbc.Spec.Config["name"])
	if err != nil {
		logger.Error(err, "failed to check if connector exists")
		return ctrl.Result{}, err
	}

	if !exists {
		// If the connector doesn't exist, create it.
		if err := r.createDebeziumConnector(dbc.Spec.DebeziumHost, dbc.Spec.Config); err != nil {
			logger.Error(err, "failed to create connector")
			return ctrl.Result{}, err
		}
		logger.Info("Debezium connector created", "name", dbc.Spec.Config["name"])
	} else {
		// The connector exists: check if its configuration matches the CR spec.
		externalConfig, err := r.getDebeziumConnectorConfig(dbc.Spec.DebeziumHost, dbc.Spec.Config["name"])
		if err != nil {
			logger.Error(err, "failed to get external connector configuration")
			return ctrl.Result{}, err
		}
		if !util.ConfigsEqual(externalConfig, dbc.Spec.Config) {
			// External configuration does not match; update it to match the CR.
			if err := r.updateDebeziumConnector(dbc.Spec.DebeziumHost, dbc.Spec.Config); err != nil {
				logger.Error(err, "failed to update connector")
				return ctrl.Result{}, err
			}
			logger.Info("Debezium connector updated to match CR", "name", dbc.Spec.Config["name"])
		}
	}

	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

// connectorExists checks if a connector with the given name exists on the Debezium host.
func (r *DebeziumConnectorReconciler) connectorExists(host, name string) (bool, error) {
	url := fmt.Sprintf("%s/connectors/%s", host, name)
	resp, err := r.HTTPClient.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// A 200 status indicates the connector exists.
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	// A 404 status indicates it does not exist.
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	// For any other status, read the response for debugging.
	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("unexpected response: %d, body: %s", resp.StatusCode, string(body))
}

// getDebeziumConnectorConfig sends a GET request to retrieves the current configuration.
func (r *DebeziumConnectorReconciler) getDebeziumConnectorConfig(host, name string) (map[string]string, error) {
	url := fmt.Sprintf("%s/connectors/%s/config", host, name)
	resp, err := r.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to GET connector config: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET connector config returned status %d: %s", resp.StatusCode, string(body))
	}
	var config map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode connector config: %w", err)
	}
	return config, nil
}

// createDebeziumConnector sends a POST request to create a new connector.
func (r *DebeziumConnectorReconciler) createDebeziumConnector(host string, config map[string]string) error {
	url := fmt.Sprintf("%s/connectors", host)

	payload := map[string]interface{}{
		"name":   config["name"],
		"config": config,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := r.HTTPClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Accept either 201 (Created) or 200 (OK) as successful responses.
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create connector, status: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

// updateDebeziumConnector sends a PUT request to update the connector configuration.
func (r *DebeziumConnectorReconciler) updateDebeziumConnector(host string, config map[string]string) error {
	url := fmt.Sprintf("%s/connectors/%s/config", host, config["name"])
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update connector, status: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

// deleteDebeziumConnector sends a DELETE request to remove the connector.
func (r *DebeziumConnectorReconciler) deleteDebeziumConnector(host, name string) error {
	url := fmt.Sprintf("%s/connectors/%s", host, name)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete connector, status: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DebeziumConnectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.DebeziumConnector{}).
		Complete(r)
}
