package v1alpha1

import (
	"fmt"

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

// ValidateCreate implements admission.Validator so a webhook will be registered for create operations.
func (r *DebeziumConnector) ValidateCreate() (admission.Warnings, error) {
	err := r.validateDebeziumConnector()
	return nil, err
}

// ValidateUpdate implements admission.Validator so a webhook will be registered for update operations.
func (r *DebeziumConnector) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	err := r.validateDebeziumConnector()
	return nil, err
}

// ValidateDelete implements admission.Validator so a webhook will be registered for delete operations.
func (r *DebeziumConnector) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

// validateDebeziumConnector contains the shared validation logic for create and update.
func (r *DebeziumConnector) validateDebeziumConnector() error {
	var allErrs field.ErrorList

	// Validate that DebeziumHost is provided.
	if r.Spec.DebeziumHost == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("debeziumHost"), "debeziumHost cannot be empty"))
	}

	// Validate that the config contains mandatory fields.
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
