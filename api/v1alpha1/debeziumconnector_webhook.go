package v1alpha1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	admission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Ensure that DebeziumConnector implements the admission.Validator interface.
var _ admission.Validator = &DebeziumConnector{}

// SetupWebhookWithManager sets up the webhook with the Manager.
func (r *DebeziumConnector) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// ValidateCreate implements admission.Validator for create operations.
func (r *DebeziumConnector) ValidateCreate() (admission.Warnings, error) {
	return nil, r.validateDebeziumConnector()
}

// ValidateUpdate implements admission.Validator for update operations.
func (r *DebeziumConnector) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	return nil, r.validateDebeziumConnector()
}

// ValidateDelete implements admission.Validator for delete operations.
func (r *DebeziumConnector) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

// validateDebeziumConnector validates the configuration of a DebeziumConnector CR.
// It performs minimal local checks and then delegates to the Debezium Connect validation endpoint.
func (r *DebeziumConnector) validateDebeziumConnector() error {
	var allErrs field.ErrorList

	// Minimal local validation: DebeziumHost must be provided.
	if r.Spec.DebeziumHost == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("debeziumHost"), "debeziumHost cannot be empty"))
	}

	// Ensure that connector.class is present (required for calling the endpoint).
	connectorClass, ok := r.Spec.Config["connector.class"]
	if !ok {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("config").Child("connector.class"), "config must include key \"connector.class\""))
	}

	// If minimal checks fail, return errors without calling the external endpoint.
	if len(allErrs) > 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("DebeziumConnector").GroupKind(), r.Name, allErrs)
	}

	// Construct the URL for the Debezium Connect validation endpoint.
	// It is assumed that r.Spec.DebeziumHost includes the protocol and port (e.g., "http://localhost:8083").
	validateURL := fmt.Sprintf("%s/connector-plugins/%s/config/validate", r.Spec.DebeziumHost, connectorClass)

	// Prepare payload for the validation endpoint.
	payload := map[string]interface{}{
		"name":   r.Spec.Config["name"],
		"config": r.Spec.Config,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal config payload: %v", err)
	}

	// Create an HTTP client with a timeout.
	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", validateURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error calling Debezium validation endpoint: %v", err)
	}
	defer resp.Body.Close()

	// Read and parse the response.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read validation response: %v", err)
	}

	// Check for non-success HTTP response.
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("debezium validation endpoint returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the validation response.
	// It is assumed that the response is a JSON object with an "errors" field.
	var validationResp struct {
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &validationResp); err != nil {
		return fmt.Errorf("failed to unmarshal validation response: %v", err)
	}

	// If the external endpoint reports any errors, aggregate them.
	if len(validationResp.Errors) > 0 {
		for key, msg := range validationResp.Errors {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("config").Child(key), r.Spec.Config[key], msg))
		}
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("DebeziumConnector").GroupKind(), r.Name, allErrs)
	}

	return nil
}
