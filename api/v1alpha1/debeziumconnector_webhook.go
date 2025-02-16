package v1alpha1

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
)

// SetupWebhookWithManager sets up the webhook with the Manager.
func (r *DebeziumConnector) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *DebeziumConnector) ValidateCreate() error {
	return r.validateDebeziumConnector()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *DebeziumConnector) ValidateUpdate(old runtime.Object) error {
	return r.validateDebeziumConnector()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *DebeziumConnector) ValidateDelete() error {
	return nil
}

// validateDebeziumConnector contains the shared validation logic for create and update.
func (r *DebeziumConnector) validateDebeziumConnector() error {
	var allErrs field.ErrorList

	// Validate that DebeziumHost is provided
	if r.Spec.DebeziumHost == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("debeziumHost"), "debeziumHost cannot be empty"))
	}

	// Validate that the config contains mandatory fields
	if len(r.Spec.Config) == 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("config"), r.Spec.Config, "config cannot be empty"))
	} else {
		requiredKeys := []string{"name", "connector.class"}
		for _, key := range requiredKeys {
			if _, ok := r.Spec.Config[key]; !ok {
				allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("config").Child(key), fmt.Sprintf("config must include key %q", key)))
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("DebeziumConnector").GroupKind(), r.Name, allErrs)
}
