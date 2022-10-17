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
	"fmt"
	"strings"

	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/affinity"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/ovs-operator/api/v1beta1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getNetworksList(
	instance *v1beta1.OVS,
) string {
	networks := "["
	for phys_net := range instance.Spec.NicMappings {
		networks += fmt.Sprintf(
			`{"name": "%s", "namespace": "%s"},`,
			phys_net, instance.Namespace,
		)
	}
	networks = strings.TrimSuffix(networks, ",")
	networks += "]"
	return networks
}

func getPhysicalNetworks(
	instance *v1beta1.OVS,
) string {
	physNets := ""
	for physNet := range instance.Spec.NicMappings {
		// NOTE(slaweq): to make things easier, each physical bridge will have
		//               the same name as "br-<physical network>"
		// NOTE(slaweq): interface names aren't important as inside Pod they will have
		//               names like "net1, net2..." so only order is important really
		physNets += fmt.Sprintf("%s ", physNet)
	}
	physNets = strings.TrimSuffix(physNets, ",")
	return physNets
}

// Deployment func
func Deployment(
	instance *v1beta1.OVS,
	configHash string,
	labels map[string]string,
) *appsv1.Deployment {

	runAsUser := int64(0)
	privileged := true
	args := []string{}

	//
	// https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/
	//
	ovsDbLivenessProbe := &corev1.Probe{
		// TODO might need tuning
		TimeoutSeconds:      5,
		PeriodSeconds:       3,
		InitialDelaySeconds: 3,
	}
	ovsDbLivenessProbe.Exec = &corev1.ExecAction{
		Command: []string{
			"/usr/bin/ovs-vsctl",
			"show",
		},
	}
	ovsVswitchdLivenessProbe := &corev1.Probe{
		// TODO might need tuning
		TimeoutSeconds:      5,
		PeriodSeconds:       3,
		InitialDelaySeconds: 3,
	}
	ovsVswitchdLivenessProbe.Exec = &corev1.ExecAction{
		Command: []string{
			"/usr/bin/ovs-appctl",
			"bond/show",
		},
	}

	envVars := map[string]env.Setter{}
	envVars["KOLLA_CONFIG_FILE"] = env.SetValue(KollaConfigAPI)
	envVars["KOLLA_CONFIG_STRATEGY"] = env.SetValue("COPY_ALWAYS")
	envVars["CONFIG_HASH"] = env.SetValue(configHash)
	envVars["OvnBridge"] = env.SetValue(instance.Spec.ExternalIDS.OvnBridge)
	envVars["OvnRemote"] = env.SetValue(instance.Spec.ExternalIDS.OvnRemote)
	envVars["OvnEncapType"] = env.SetValue(instance.Spec.ExternalIDS.OvnEncapType)
	envVars["OvnEncapIP"] = env.SetValue(instance.Spec.ExternalIDS.OvnEncapIP)
	envVars["EnableChassisAsGateway"] = env.SetValue(fmt.Sprintf("%t", instance.Spec.ExternalIDS.EnableChassisAsGateway))
	envVars["PhysicalNetworks"] = env.SetValue(getPhysicalNetworks(instance))

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName,
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: &instance.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"k8s.v1.cni.cncf.io/networks": getNetworksList(instance),
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ServiceAccountName,
					Containers: []corev1.Container{
						// ovsdb-server container
						{
							Name: ServiceName + "db-server",
							Command: []string{
								"/usr/bin/start-ovs",
								"ovsdb-server",
							},
							Args:  args,
							Image: instance.Spec.OvsContainerImage,
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add:  []corev1.Capability{"NET_ADMIN", "SYS_ADMIN", "SYS_NICE"},
									Drop: []corev1.Capability{},
								},
								RunAsUser:  &runAsUser,
								Privileged: &privileged,
							},
							Env:           env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:  GetOvsDbVolumeMounts(),
							Resources:     instance.Spec.Resources,
							LivenessProbe: ovsDbLivenessProbe,
						}, {
							// ovs-vswitchd container
							Name: ServiceName + "-vswitchd",
							Command: []string{
								"/usr/bin/start-ovs",
								"ovs-vswitchd",
							},
							Args:  args,
							Image: instance.Spec.OvsContainerImage,
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add:  []corev1.Capability{"NET_ADMIN", "SYS_ADMIN", "SYS_NICE"},
									Drop: []corev1.Capability{},
								},
								RunAsUser:  &runAsUser,
								Privileged: &privileged,
							},
							Env:           env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:  GetVswitchdVolumeMounts(),
							Resources:     instance.Spec.Resources,
							LivenessProbe: ovsVswitchdLivenessProbe,
						}, {
							// ovn-controller container
							Name: OvnControllerServiceName,
							Command: []string{
								"/bin/bash", "-c",
							},
							Args: []string{
								// First configure external ids
								"/usr/local/bin/container-scripts/init.sh && /usr/bin/ovn-controller --pidfile --log-file unix:/run/openvswitch/db.sock",
							},
							Image: instance.Spec.OvnContainerImage,
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add:  []corev1.Capability{"NET_ADMIN", "SYS_ADMIN", "SYS_NICE"},
									Drop: []corev1.Capability{},
								},
								RunAsUser:  &runAsUser,
								Privileged: &privileged,
							},
							Env:          env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts: GetOvnVolumeMounts(),
							Resources:    instance.Spec.Resources,
						},
					},
				},
			},
		},
	}
	deployment.Spec.Template.Spec.Volumes = GetVolumes(instance.Name)
	// If possible two pods of the same service should not
	// run on the same worker node. If this is not possible
	// the get still created on the same worker node.
	deployment.Spec.Template.Spec.Affinity = affinity.DistributePods(
		common.AppSelector,
		[]string{
			ServiceName,
		},
		corev1.LabelHostname,
	)
	if instance.Spec.NodeSelector != nil && len(instance.Spec.NodeSelector) > 0 {
		deployment.Spec.Template.Spec.NodeSelector = instance.Spec.NodeSelector
	}

	return deployment

}
