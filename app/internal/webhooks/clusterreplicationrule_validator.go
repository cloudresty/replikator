package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"replikator/apis/replication/v1"
)

type ClusterReplicationRuleValidator struct{}

func NewClusterReplicationRuleValidator() *ClusterReplicationRuleValidator {
	return &ClusterReplicationRuleValidator{}
}

var _ admission.CustomValidator = &ClusterReplicationRuleValidator{}

func (v *ClusterReplicationRuleValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	crr, ok := obj.(*v1.ClusterReplicationRule)
	if !ok {
		return nil, fmt.Errorf("expected ClusterReplicationRule but got %T", obj)
	}

	if err := validateClusterReplicationRule(crr); err != nil {
		return nil, err
	}

	return nil, nil
}

func (v *ClusterReplicationRuleValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	crr, ok := newObj.(*v1.ClusterReplicationRule)
	if !ok {
		return nil, fmt.Errorf("expected ClusterReplicationRule but got %T", newObj)
	}

	if err := validateClusterReplicationRule(crr); err != nil {
		return nil, err
	}

	return nil, nil
}

func (v *ClusterReplicationRuleValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *ClusterReplicationRuleValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	switch req.Operation {
	case "CREATE", "UPDATE":
		obj := &v1.ClusterReplicationRule{}
		if err := json.Unmarshal(req.Object.Raw, obj); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if err := validateClusterReplicationRule(obj); err != nil {
			return admission.Denied(err.Error())
		}

		return admission.Allowed("validation passed")

	case "DELETE":
		return admission.Allowed("deletion allowed")

	default:
		return admission.Allowed("operation not requiring validation")
	}
}

func validateClusterReplicationRule(crr *v1.ClusterReplicationRule) error {
	var validationErrors []string

	if crr.Spec.SourceNamespace == "" {
		validationErrors = append(validationErrors, "spec.sourceNamespace is required")
	}

	if crr.Spec.SourceSelector == "" {
		validationErrors = append(validationErrors, "spec.sourceSelector is required")
	} else {
		if err := validateSelector(crr.Spec.SourceSelector); err != nil {
			validationErrors = append(validationErrors, err.Error())
		}
	}

	if crr.Spec.TargetNamespaces.Selector != "" {
		if err := validateNamespacePattern(crr.Spec.TargetNamespaces.Selector); err != nil {
			validationErrors = append(validationErrors, err.Error())
		}
	}

	if len(crr.Spec.ResourceTypes) > 0 {
		for _, rt := range crr.Spec.ResourceTypes {
			if err := validateResourceType(rt); err != nil {
				validationErrors = append(validationErrors, err.Error())
			}
		}
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("validation failed: %s", joinErrors(validationErrors))
	}

	return nil
}

func joinErrors(errors []string) string {
	result := ""
	for i, e := range errors {
		if i > 0 {
			result += "; "
		}
		result += e
	}
	return result
}

func validateSelector(selector string) error {
	if selector == "" {
		return fmt.Errorf("selector cannot be empty")
	}

	regexStr := selector

	if !strings.HasPrefix(selector, "^") && !strings.HasPrefix(selector, ".*") {
		regexStr = "^" + regexStr
	}
	if !strings.HasSuffix(selector, "$") && !strings.HasSuffix(selector, ".*") {
		regexStr = regexStr + "$"
	}

	_, err := regexp.Compile(regexStr)
	return err
}

func validateNamespacePattern(pattern string) error {
	if pattern == "" {
		return nil
	}

	return validateSelector(pattern)
}

func validateResourceType(resourceType string) error {
	switch resourceType {
	case "Secret", "ConfigMap", "Certificate", "Ingress":
		return nil
	default:
		return fmt.Errorf("unsupported resource type: %s (must be Secret, ConfigMap, Certificate, or Ingress)", resourceType)
	}
}
