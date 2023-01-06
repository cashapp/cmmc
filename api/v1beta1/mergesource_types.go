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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cashapp/cmmc/util"
	"github.com/pkg/errors"
)

// MergeSourceSourceSpec describes the data key of the ConfigMap we
// choose to aggregate from for this MergeSource.
type MergeSourceSourceSpec struct {
	Data string `json:"data,omitempty"`
}

// MergeSourceTargetSpec describes the MergeTarget a MergeSource will target.
type MergeSourceTargetSpec struct {
	// Name specifies the MergeTarget we will attempt to write to (if it exists).
	Name string `json:"name,omitempty"`

	// Data is the data key of the MergeTarget.
	//
	// This key must be present on the MergeTarget as well.
	Data string `json:"data,omitempty"`
}

// MergeSourceSpec defines the configuration for a MergeSource.
// Manily, which ConfigMap resources to watch, which key it will be
// aggregating data from, and which MergeTarget it will be writing to.
type MergeSourceSpec struct {
	// Selector specifies what labels on a source ConfigMap the controller will be watching.
	Selector map[string]string `json:"selector,omitempty"`

	// NamespaceSelector specifies what lables _must be_ on the source ConfigMaps namespace,
	// (if any) for this to become a valid source.
	//
	// If omitted, will allow ConfigMaps from all namespaces.
	NamespaceSelector map[string]string `json:"namespaceSelector,omitempty"`

	// Source describes which data key from the source ConfigMap we will be observing/merging.
	Source MergeSourceSourceSpec `json:"source,omitempty"`

	// Target is where the aggregated data for this source will be written.
	Target MergeSourceTargetSpec `json:"target,omitempty"`
}

// MergeSourceStatus defines the observed state of MergeSource.
type MergeSourceStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	Output     string             `json:"output,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MergeSource is the Schema for the mergesources API.
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""
type MergeSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MergeSourceSpec   `json:"spec,omitempty"`
	Status MergeSourceStatus `json:"status,omitempty"`
}

// NamespaceSelector gives us the NamespaceSelector for the MergeSource
func (m *MergeSource) NamespaceSelector() map[string]string {
	return m.Spec.NamespaceSelector
}

// NamespacedTargetName gets the types.NamespacedName representation given the
// namespace of the MergeSource resource.
func (m *MergeSource) NamespacedTargetName() (types.NamespacedName, error) {
	n, err := util.NamespacedName(m.Spec.Target.Name, m.Namespace)
	return n, errors.WithStack(err)
}

func (m *MergeSource) SetStatusCondition(c metav1.Condition) {
	meta.SetStatusCondition(&m.Status.Conditions, c)
}

func (m *MergeSource) FindStatusCondition(conditionType string) *metav1.Condition {
	return meta.FindStatusCondition(m.Status.Conditions, conditionType)
}

func NewMergeSource(n types.NamespacedName, spec MergeSourceSpec) *MergeSource {
	return &MergeSource{
		TypeMeta:   metav1.TypeMeta{APIVersion: GroupVersion.String(), Kind: "MergeSource"},
		ObjectMeta: metav1.ObjectMeta{Name: n.Name, Namespace: n.Namespace},
		Spec:       spec,
	}
}

func MergeSourceNamespacedTargetName(o client.Object) (types.NamespacedName, bool) {
	source, ok := o.(*MergeSource)
	if !ok {
		return types.NamespacedName{}, false
	}

	n, err := source.NamespacedTargetName()
	if err != nil {
		return n, false
	}

	return n, true
}

//+kubebuilder:object:root=true

// MergeSourceList contains a list of MergeSource.
type MergeSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MergeSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MergeSource{}, &MergeSourceList{})
}
