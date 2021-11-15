package v1beta1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MergeSourceConditionReady(numSources int) metav1.Condition {
	return metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "outputAccumulated",
		Message: fmt.Sprintf("Data from %d ConfigMap(s) accumulated.", numSources),
	}
}

func MergeTargetConditionValidationErrors(numSources int, errors []string) metav1.Condition {
	return metav1.Condition{
		Type:    "cmmc/Validation",
		Status:  metav1.ConditionFalse,
		Reason:  "validationErrors",
		Message: fmt.Sprintf("%d MergeSources reporting validation errors: %s", numSources, errors),
	}
}

func MergeTargetConditionNoValidationErrors(numSources int) metav1.Condition {
	return metav1.Condition{
		Type:    "cmmc/Validation",
		Status:  metav1.ConditionTrue,
		Reason:  "noValidationErrors",
		Message: fmt.Sprintf("%d MergeSources reporting valid data.", numSources),
	}
}

func MergeTargetConditionMissingTarget(err error) metav1.Condition {
	return metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Reason:  "invalidTarget",
		Message: fmt.Sprintf("Invalid spec.target %s", err.Error()),
	}
}

func MergeTargetConditionErrorUpdating(err error, numUpdatedKeys int) metav1.Condition {
	return metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Reason:  "errorUpdating",
		Message: fmt.Sprintf("Failed to update %d keys in target ConfigMap: %s", numUpdatedKeys, err.Error()),
	}
}

func MergeTargetConditionPartialUpdate() metav1.Condition {
	return metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionUnknown,
		Reason:  "partialUpdate",
		Message: "Target possibly partially updated. See validation condition for errors.",
	}
}

func MergeTargetConditionUpdated() metav1.Condition {
	return metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "targetUpdated",
		Message: "Target ConfigMap up to date.",
	}
}

func MergeTargetConditionReady(hasErrors bool) metav1.Condition {
	if hasErrors {
		return MergeTargetConditionPartialUpdate()
	}

	return MergeTargetConditionUpdated()
}

func MergeTargetConditionValidation(errors []string, numSources int) metav1.Condition {
	if len(errors) > 0 {
		return MergeTargetConditionValidationErrors(numSources, errors)
	}

	return MergeTargetConditionNoValidationErrors(numSources)
}
