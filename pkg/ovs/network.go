/*
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

package ovs

import (
	"context"
	"fmt"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openstack-k8s-operators/ovs-operator/api/v1alpha1"
)

func CreateAdditionalNetworks(
	instance *v1alpha1.OVS,
	labels map[string]string,
	ctx context.Context,
	k8sClient client.Client,
) error {

	var nad *netattdefv1.NetworkAttachmentDefinition

	for phys_net, interfaceName := range instance.Spec.NicMappings {
		nad = &netattdefv1.NetworkAttachmentDefinition{}
		err := k8sClient.Get(
			ctx,
			client.ObjectKey{
				Namespace: instance.Namespace,
				Name:      phys_net,
			},
			nad,
		)
		if err != nil {
			if !k8s_errors.IsNotFound(err) {
				return fmt.Errorf("can not get NetworkAttachmentDefinition %s/%s: %w",
					phys_net, interfaceName, err)
			}

			nad = &netattdefv1.NetworkAttachmentDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:      phys_net,
					Namespace: instance.Namespace,
					Labels:    labels,
				},
				Spec: netattdefv1.NetworkAttachmentDefinitionSpec{
					Config: fmt.Sprintf(
						`{"cniVersion": "0.3.1", "name": "%s", "type": "host-device", "device": "%s"}`,
						phys_net, interfaceName),
				},
			}
			// Request object not found, lets create it
			if err := k8sClient.Create(ctx, nad); err != nil {
				return fmt.Errorf("can not create NetworkAttachmentDefinition %s/%s: %w",
					phys_net, interfaceName, err)
			}
		}
	}
	return nil
}