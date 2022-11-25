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
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/ovs-operator/api/v1beta1"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
)

type network struct {
	Name      string
	Namespace string
}

func getNetworksList(
	instance *v1beta1.OVS,
) (string, error) {
	networksList := []network{}
	if instance.Spec.TunnelNetworkNic != "" {
		networksList = append(
			networksList,
			network{
				Name:      TunnelNetworkName,
				Namespace: instance.Namespace,
			},
		)
	}

	for physNet := range instance.Spec.NicMappings {
		networksList = append(
			networksList,
			network{
				Name:      physNet,
				Namespace: instance.Namespace,
			},
		)
	}
	networks, err := json.Marshal(networksList)
	if err != nil {
		return "", fmt.Errorf("failed to encode networks %s into json: %w",
			networksList, err)
	}
	return string(networks), nil
}

func getPhysicalNetworks(
	instance *v1beta1.OVS,
) string {
	// NOTE(slaweq): to make things easier, each physical bridge will have
	//               the same name as "br-<physical network>"
	// NOTE(slaweq): interface names aren't important as inside Pod they will have
	//               names like "net1, net2..." so only order is important really
	return strings.Join(
		maps.Keys(instance.Spec.NicMappings), " ",
	)
}

// GetIPFromCidr - get IP address from given CIDR
func GetIPFromCidr(cidr string) (string, error) {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("failed to parse CIDR %s: %w", cidr, err)
	}
	return ip.String(), nil
}

// EnvDownwardAPI - set env from FieldRef->FieldPath, e.g. status.podIP
func EnvDownwardAPI(field string) env.Setter {
	return func(env *corev1.EnvVar) {
		if env.ValueFrom == nil {
			env.ValueFrom = &corev1.EnvVarSource{}
		}
		env.Value = ""

		if env.ValueFrom.FieldRef == nil {
			env.ValueFrom.FieldRef = &corev1.ObjectFieldSelector{}
		}

		env.ValueFrom.FieldRef.FieldPath = field
	}
}
