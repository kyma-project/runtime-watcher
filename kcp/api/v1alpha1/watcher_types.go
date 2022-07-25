/*
Copyright 2022.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WatcherSpec defines the desired state of Watcher
type WatcherSpec struct {
	// Watcher URL consists of <IngressEndpoint>:<IngressPort>/ContractVersion/ComponentName

	// ContractVersion signifies the contract version appended to the Watcher endpoint.
	ContractVersion string `json:"contractVersion"`

	// ComponentName signifies the component name appended to the Watcher endpoint.
	ComponentName string `json:"componentName"`

	// ServiceInfo describes the service information of the operator
	ServiceInfo ServiceInfo `json:"serviceInfo"`
}

type ServiceInfo struct {
	// ServicePort describes the port on which operator service can be reached.
	ServicePort int64 `json:"servicePort"`

	// ServiceName describes the service name for the operator.
	ServiceName string `json:"serviceName"`
}

// +kubebuilder:validation:Enum=Processing;Deleting;Ready;Error
type WatcherState string

// Valid Watcher States.
const (
	// WatcherStateReady signifies Watcher is ready and has been installed successfully.
	WatcherStateReady WatcherState = "Ready"

	// WatcherStateProcessing signifies Watcher is reconciling and is in the process of installation.
	WatcherStateProcessing WatcherState = "Processing"

	// WatcherStateError signifies an error for Watcher. This signifies that the Installation
	// process encountered an error.
	WatcherStateError WatcherState = "Error"

	// WatcherStateDeleting signifies Watcher is being deleted.
	WatcherStateDeleting WatcherState = "Deleting"
)

// WatcherStatus defines the observed state of Watcher.
type WatcherStatus struct {
	// State signifies current state of a Watcher.
	// Value can be one of ("Ready", "Processing", "Error", "Deleting")
	State WatcherState `json:"state"`

	// List of status conditions to indicate the status of a Watcher.
	// +kubebuilder:validation:Optional
	Conditions []WatcherCondition `json:"conditions"`
}

// WatcherCondition describes condition information for Watcher
type WatcherCondition struct {
	// Type is used to reflect what type of condition we are dealing with. Most commonly ConditionTypeReady it is used
	// as extension marker in the future
	Type WatcherConditionStatus `json:"type"`

	// Status of the Watcher Condition.
	// Value can be one of ("True", "False", "Unknown").
	Status WatcherConditionStatus `json:"status"`

	// Human-readable message indicating details about the last status transition.
	// +kubebuilder:validation:Optional
	Message string `json:"message"`

	// Machine-readable text indicating the reason for the condition's last transition.
	// +kubebuilder:validation:Optional
	Reason string `json:"reason"`

	// Timestamp for when Watcher last transitioned from one status to another.
	// +kubebuilder:validation:Optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime"`
}

type WatcherConditionType string

const (
	// ConditionTypeReady represents WatcherConditionType Ready, meaning as soon as its true we will reconcile Watcher
	// into WatcherStateReady.
	ConditionTypeReady WatcherConditionType = "Ready"
)

type WatcherConditionStatus string

// Valid WatcherConditionStatus.
const (
	// ConditionStatusTrue signifies WatcherConditionStatus true.
	ConditionStatusTrue WatcherConditionStatus = "True"

	// ConditionStatusFalse signifies WatcherConditionStatus false.
	ConditionStatusFalse WatcherConditionStatus = "False"

	// ConditionStatusUnknown signifies WatcherConditionStatus unknown.
	ConditionStatusUnknown WatcherConditionStatus = "Unknown"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Watcher is the Schema for the watchers API
type Watcher struct {
	metav1.TypeMeta `json:",inline"`

	// +kubebuilder:validation:Optional
	metav1.ObjectMeta `json:"metadata"`

	// +kubebuilder:validation:Optional
	Spec WatcherSpec `json:"spec"`

	// +kubebuilder:validation:Optional
	Status WatcherStatus `json:"status"`
}

//+kubebuilder:object:root=true

// WatcherList contains a list of Watcher
type WatcherList struct {
	metav1.TypeMeta `json:",inline"`

	// +kubebuilder:validation:Optional
	metav1.ListMeta `json:"metadata"`
	Items           []Watcher `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Watcher{}, &WatcherList{})
}
