package ovs

import (
	corev1 "k8s.io/api/core/v1"
)

// InitContainer information
type InitContainer struct {
	Privileged     bool
	ContainerImage string
	SystemID       string
	Hostname       string
	OvnBridge      string
	OvnRemote      string
	OvnEncapType   string
	OvnEncapIP     string
	VolumeMounts   []corev1.VolumeMount
}

// GetInitContainer - init container for Open vSwitch services
func GetInitContainer(init InitContainer) []corev1.Container {
	runAsUser := int64(0)
	trueVar := true

	securityContext := &corev1.SecurityContext{
		RunAsUser: &runAsUser,
	}
	if init.Privileged {
		securityContext.Privileged = &trueVar
	}

	return []corev1.Container{
		{
			Name:            "init",
			Image:           init.ContainerImage,
			SecurityContext: securityContext,
			Command: []string{
				"/bin/bash", "-c", "/usr/local/bin/container-scripts/init.sh",
			},
			Env: []corev1.EnvVar{
				{
					Name:  "Hostname",
					Value: init.Hostname,
				},
				{
					Name:  "OvnBridge",
					Value: init.OvnBridge,
				},
				{
					Name:  "OvnRemote",
					Value: init.OvnRemote,
				},
				{
					Name:  "OvnEncapType",
					Value: init.OvnEncapType,
				},
				{
					Name:  "OvnEncapIP",
					Value: init.OvnEncapIP,
				},
			},
			VolumeMounts: init.VolumeMounts,
		},
	}
}
