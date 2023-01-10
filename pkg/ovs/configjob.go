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

	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/ovs-operator/api/v1beta1"

	ovnclient "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigJob - prepare job to configure ovn-controller
func ConfigJob(
	ctx context.Context,
	h *helper.Helper,
	instance *v1beta1.OVS,
	configHash string,
	labels map[string]string,
) (*batchv1.Job, error) {

	runAsUser := int64(0)
	privileged := true
	jobTTLAfterFinished := int32(10)

	dbmap, err := ovnclient.GetDBEndpoints(ctx, h, instance.Namespace, map[string]string{})
	if err != nil {
		return nil, err
	}

	envVars := map[string]env.Setter{}
	envVars["KOLLA_CONFIG_FILE"] = env.SetValue(KollaConfigAPI)
	envVars["KOLLA_CONFIG_STRATEGY"] = env.SetValue("COPY_ALWAYS")
	envVars["CONFIG_HASH"] = env.SetValue(configHash)
	envVars["OvnBridge"] = env.SetValue(instance.Spec.ExternalIDS.OvnBridge)
	envVars["OvnRemote"] = env.SetValue(dbmap["SB"])
	envVars["OvnEncapType"] = env.SetValue(instance.Spec.ExternalIDS.OvnEncapType)
	envVars["OvnEncapIP"] = EnvDownwardAPI("status.podIP")
	envVars["EnableChassisAsGateway"] = env.SetValue(fmt.Sprintf("%t", instance.Spec.ExternalIDS.EnableChassisAsGateway))
	envVars["PhysicalNetworks"] = env.SetValue(getPhysicalNetworks(instance))
	envVars["OvnHostName"] = EnvDownwardAPI("spec.nodeName")

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-configuration",
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &jobTTLAfterFinished,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      "OnFailure",
					ServiceAccountName: ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  instance.Name + "-ovn-configuration-sync",
							Image: instance.Spec.OvnContainerImage,
							Command: []string{
								"/bin/bash", "-c",
							},
							Args: []string{
								"/usr/local/bin/container-scripts/init.sh",
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Add:  []corev1.Capability{"NET_ADMIN", "SYS_ADMIN"},
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
					Volumes: GetVolumes(instance.Name),
				},
			},
		},
	}

	if instance.Spec.NodeSelector != nil && len(instance.Spec.NodeSelector) > 0 {
		job.Spec.Template.Spec.NodeSelector = instance.Spec.NodeSelector
	}

	return job, nil
}
