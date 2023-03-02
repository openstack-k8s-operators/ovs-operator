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
	ExternalIDS OVSExternalIDs `json:"external-ids"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="quay.io/tripleozedcentos9/openstack-ovn-base:current-tripleo"
	// Image used for the ovsdb-server and ovs-vswitchd containers
	OvsContainerImage string `json:"ovsContainerImage"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="quay.io/tripleozedcentos9/openstack-ovn-controller:current-tripleo"
	// Image used for the ovn-controller container
	OvnContainerImage string `json:"ovnContainerImage"`

	// +kubebuilder:validation:Optional
	// +optional
	NicMappings map[string]string `json:"nicMappings,omitempty"`

	// +kubebuilder:validation:Optional
	// Resources - Compute Resources required by this service (Limits/Requests).
	// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running this service
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	// NetworkAttachment is a NetworkAttachment resource name to expose the service to the given network.
	// If specified the IP address of this network is used as the OvnEncapIP.
	NetworkAttachment string `json:"networkAttachment"`
}

// OVSStatus defines the observed state of OVS
type OVSStatus struct {
	// NumberReady of the ovs instances
	NumberReady int32 `json:"numberReady,omitempty"`

	// DesiredNumberScheduled - total number of the nodes which should be running Daemon
	DesiredNumberScheduled int32 `json:"desiredNumberScheduled,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`

	// NetworkAttachments status of the deployment pods
	NetworkAttachments map[string][]string `json:"networkAttachments,omitempty"`
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
	return instance.Status.NumberReady == instance.Status.DesiredNumberScheduled
}

// OVSExternalIDs is a set of configuration options for OVS external-ids table
type OVSExternalIDs struct {
	SystemID  string `json:"system-id"`
	OvnBridge string `json:"ovn-bridge"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default="geneve"
	// +kubebuilder:validation:Enum={"geneve","vxlan"}
	// OvnEncapType - geneve or vxlan
	OvnEncapType string `json:"ovn-encap-type,omitempty"`

	EnableChassisAsGateway bool `json:"enable-chassis-as-gateway,omitempty" optional:"true"`
}
