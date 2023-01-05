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

package v1beta1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/cashapp/cmmc/util"
	"github.com/cashapp/cmmc/util/validator"
	"github.com/pkg/errors"
)

const (
	DataNewlyCreatedStatusYes string = "YES"
	DataNewlyCreatedStatusNo  string = "NO"
)

type MergeTargetDataSpec struct {
	// +optional
	Init string `json:"init,omitempty"`

	// +optional
	JSONSchema string `json:"jsonSchema,omitempty"`
}

// MergeTargetDataStatus represents the status of the MergeTarget resource.
type MergeTargetDataStatus struct {
	// Init is the initial value of the data key (at the time that the MergeTarget came into existence).
	Init string `json:"init,omitempty"`

	// NewlyCreated is "YES" whether or not the MergeTarget created this data key.
	NewlyCreated string `json:"newlyCreated,omitempty"`
}

// IsStatusNewlyCreated returns true if this field is created by the controller.
func (m *MergeTargetDataStatus) IsStatusNewlyCreated() bool {
	return m.NewlyCreated == DataNewlyCreatedStatusYes
}

// WithMaybeUpdatedInit will produce a copy of the status with the initial state
// set to this data if it is missing (and the target is managing it).
func (m MergeTargetDataStatus) WithMaybeUpdatedInit(data string) MergeTargetDataStatus {
	c := m
	if m.NewlyCreated == DataNewlyCreatedStatusYes && m.Init != data {
		c.Init = data
	}

	return c
}

// NewlyCreatedMergeTargetDataStatus is the initial status for a newly created Target.
func NewlyCreatedMergeTargetDataStatus(init string) MergeTargetDataStatus {
	return MergeTargetDataStatus{
		NewlyCreated: DataNewlyCreatedStatusYes,
		Init:         init,
	}
}

// MergeTargetSpec defines the desired state of MergeTarget.
type MergeTargetSpec struct {
	// Target refers to the config map we are either creating, or updating.
	Target string                         `json:"target,omitempty"`
	Data   map[string]MergeTargetDataSpec `json:"data,omitempty"`
}

// MergeTargetStatus defines the observed state of MergeTarget.
type MergeTargetStatus struct {
	// NewlyCreated means that the resource is fully managing the state of this ConfigMap.
	//
	// - empty means that we've never done anything.
	// - "NO" means that the configMap was already there.
	// - "YES" means that the target createdt he configMap initially.
	NewlyCreated string `json:"newlyCreated,omitempty"`

	// Data is the status of each of the data keys that we are monitoring.
	Data map[string]MergeTargetDataStatus `json:"data,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MergeTarget is the Schema for the mergetargets API.
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=".spec.target"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
// +kubebuilder:printcolumn:name="Validation",type="string",JSONPath=".status.conditions[?(@.type==\"cmmc/Validation\")].message",description=""
type MergeTarget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MergeTargetSpec   `json:"spec,omitempty"`
	Status MergeTargetStatus `json:"status,omitempty"`
}

// NamespacedTargetName gets the namespace target name for this MergeTarget.
func (m *MergeTarget) NamespacedTargetName() (types.NamespacedName, error) {
	n, err := util.NamespacedName(m.Spec.Target, m.Namespace)

	return n, errors.WithStack(err)
}

// IsStatusNewlyCreated returns true if the contoller has created the ConfigMap
// that this MergeTarget targets.
//
// If it did, it would be safe to delete during MergeTarget cleanup.
func (m *MergeTarget) IsStatusNewlyCreated() bool {
	return m.Status.NewlyCreated == DataNewlyCreatedStatusYes
}

// SetStatusCondition sets the v1beta1.Condition.
func (m *MergeTarget) SetStatusCondition(c metav1.Condition) {
	meta.SetStatusCondition(&m.Status.Conditions, c)
}

func (m *MergeTarget) FindStatusCondition(conditionType string) *metav1.Condition {
	return meta.FindStatusCondition(m.Status.Conditions, conditionType)
}

// UpdateDataStatus updates the data/status keys of the MergeTarget depending
// the ConfigMap's data.
//
// This is critical so that the MergeTarget will know how reset the ConfigMap
// once/if it needs cleaning up, and so we know how to deterministically
// do the Merging.
func (m *MergeTarget) UpdateDataStatus(configMapData map[string]string) {
	if m.Status.Data == nil {
		m.Status.Data = map[string]MergeTargetDataStatus{}
	}

	for k, v := range m.Spec.Data {
		var (
			nextState                  MergeTargetDataStatus
			existingData, dataExists   = configMapData[k]
			existingState, stateExists = m.Status.Data[k]
		)

		if stateExists { //nolint:gocritic
			// If the state exists we are already managing this CM so
			// let's keep going by doing what we need to do regardless
			// of what the data says
			nextState = existingState.WithMaybeUpdatedInit(v.Init)
		} else if dataExists {
			// if there is no state, but there is data, we are taking over
			// an existing configMap, so lets take care of this.
			nextState = MergeTargetDataStatus{Init: existingData}
		} else {
			// we have neither state nor data, we should use the `spec` to update
			// and create the initial state
			nextState = NewlyCreatedMergeTargetDataStatus(v.Init)
		}

		// write the next status
		m.Status.Data[k] = nextState
	}
}

// ReduceDataState mutates configMapData, accumulating the MergeSourceList into the respective keys.
//
//nolint:cyclop
func (m *MergeTarget) ReduceDataState(
	mergeSources MergeSourceList, configMapData *map[string]string,
) (statusKeysToRemove []string, updatedKeys int, fieldsErrors []string) {
	configMap := *configMapData

	for k, v := range m.Status.Data {
		//
		// If the Spec for the MergeTarget no longer has the key
		// specified, we revert the configMap to its original state,
		// either the initial data for that key, or removing it entirely.
		//
		// This will end up keeping the status key, which we want to do
		// until we are confident that the CM has been reverted successfully.
		if _, ok := m.Spec.Data[k]; !ok {
			existingValue, exists := configMap[k]
			if !exists && v.IsStatusNewlyCreated() {
				// do nothing, this is all good, it doesn't exist
				// and it was supposed to be newly created/managed by the MergeTarget
			} else if existingValue != v.Init {
				configMap[k] = v.Init
				updatedKeys++
			}

			statusKeysToRemove = append(statusKeysToRemove, k)
			continue
		}

		//
		// create & aggregate the data from the mergeSources
		data := v.Init
		for _, source := range mergeSources.Items {
			if source.Spec.Target.Data == k {
				data += source.Status.Output
			}
		}

		// possibly validate the field if JSONSchema was specified
		// N.B. we _allow empty here_!
		if m.Spec.Data[k].JSONSchema != "" && data != "" {
			if err := validator.Validate(m.Spec.Data[k].JSONSchema, data); err != nil {
				fieldsErrors = append(fieldsErrors, fmt.Sprintf("%s: %s", k, err.Error()))
				continue
			}
		}

		if configMap == nil {
			configMap = map[string]string{}
		}

		existingData := configMap[k]
		if existingData != data {
			configMap[k] = data
			updatedKeys++
		}
	}

	*configMapData = configMap

	return statusKeysToRemove, updatedKeys, fieldsErrors
}

func (m *MergeTarget) RemoveDataStatusKeys(keys []string) {
	for _, k := range keys {
		delete(m.Status.Data, k)
	}
}

func NewMergeTarget(name types.NamespacedName, spec MergeTargetSpec) *MergeTarget {
	return &MergeTarget{
		TypeMeta: metav1.TypeMeta{
			APIVersion: GroupVersion.String(),
			Kind:       "MergeTarget",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Spec: spec,
	}
}

//+kubebuilder:object:root=true

// MergeTargetList contains a list of MergeTarget.
type MergeTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MergeTarget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MergeTarget{}, &MergeTargetList{})
}
