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

package v1beta1

import (
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OVSSpec defines the desired state of OVS
type OVSSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Maximum=1
	// +kubebuilder:validation:Minimum=0
	// Replicas of OVS to run
	Replicas int32 `json:"replicas,omitempty"`

	// +kubebuilder:validation:Optional
	ExternalIDS OVSExternalIDs `json:"external-ids"`

	OvsContainerImage string `json:"ovsContainerImage,omitempty"`

	OvnContainerImage string `json:"ovnContainerImage,omitempty"`

	// +kubebuilder:validation:Optional
	NicMappings map[string]string `json:"nic_mappings,omitempty"`

	// +kubebuilder:validation:Optional
	// Resources - Compute Resources required by this service (Limits/Requests).
	// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running this service
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// OVSStatus defines the observed state of OVS
type OVSStatus struct {
	// ReadyCount of the ovs instances
	ReadyCount int32 `json:"readyCount,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OVS is the Schema for the ovs API
type OVS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OVSSpec   `json:"spec,omitempty"`
	Status OVSStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OVSList contains a list of OVS
type OVSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OVS `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OVS{}, &OVSList{})
}

// IsReady - returns true if service is ready to server requests
func (instance OVS) IsReady() bool {
	// Ready when:
	// there is at least a single pod to running OVS and ovn-controller
	return instance.Status.ReadyCount >= 1
}

// OVSExternalIDs is a set of configuration options for OVS external-ids table
type OVSExternalIDs struct {
	SystemID               string `json:"system-id"`
	Hostname               string `json:"hostname"`
	OvnBridge              string `json:"ovn-bridge"`
	OvnRemote              string `json:"ovn-remote"`
	OvnEncapType           string `json:"ovn-encap-type"`
	OvnEncapIP             string `json:"ovn-encap-ip"`
	EnableChassisAsGateway bool   `json:"enable-chassis-as-gateway" optional:"false"`
}
