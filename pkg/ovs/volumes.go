package ovs

import (
	corev1 "k8s.io/api/core/v1"
)

/*
NOTE(slaweq): this is used only for InitContainer, so maybe we don't need it here
// GetInitVolumeMounts -
func GetInitVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{}

}
*/

// GetVolumes -
// TODO: merge to GetVolumes when other controllers also switched to current config
//       mechanism.
func GetVolumes(name string) []corev1.Volume {
	//source_type := corev1.HostPathDirectoryOrCreate
	return []corev1.Volume{
		{
			Name: "etc-machine-id",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/machine-id",
				},
			},
		},
		{
			Name: "etc-localtime",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/localtime",
				},
			},
		},
		{
			Name: "etc-ovs",
			VolumeSource: corev1.VolumeSource{
				//TODO (slaweq): it will probably need to be HostPath type but when I'm using it I got error like:
				// Creating empty database /etc/openvswitch/conf.db ovsdb-tool: I/O error: /etc/openvswitch/conf.db: failed to lock lockfile (Resource temporarily unavailable)
				// and containers aren't started
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "var-run",
			VolumeSource: corev1.VolumeSource{
				//TODO (slaweq): it will probably need to be HostPath type but when I'm using it I got error like:
				// Creating empty database /etc/openvswitch/conf.db ovsdb-tool: I/O error: /etc/openvswitch/conf.db: failed to lock lockfile (Resource temporarily unavailable)
				// and containers aren't started
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "var-log",
			VolumeSource: corev1.VolumeSource{
				//TODO (slaweq): it will probably need to be HostPath type but when I'm using it I got error like:
				// Creating empty database /etc/openvswitch/conf.db ovsdb-tool: I/O error: /etc/openvswitch/conf.db: failed to lock lockfile (Resource temporarily unavailable)
				// and containers aren't started
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "var-lib",
			VolumeSource: corev1.VolumeSource{
				//TODO (slaweq): it will probably need to be HostPath type but when I'm using it I got error like:
				// Creating empty database /etc/openvswitch/conf.db ovsdb-tool: I/O error: /etc/openvswitch/conf.db: failed to lock lockfile (Resource temporarily unavailable)
				// and containers aren't started
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

}

// GetOvsDbVolumeMounts - ovsdb-server VolumeMounts
func GetOvsDbVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "etc-machine-id",
			MountPath: "/etc/machine-id",
			ReadOnly:  true,
		},
		{
			Name:      "etc-localtime",
			MountPath: "/etc/localtime",
			ReadOnly:  true,
		},
		{
			Name:      "etc-ovs",
			MountPath: "/etc/openvswitch",
			ReadOnly:  false,
		},
		{
			Name:      "var-run",
			MountPath: "/var/run/openvswitch",
			ReadOnly:  false,
		},
		{
			Name:      "var-log",
			MountPath: "/var/log/openvswitch",
			ReadOnly:  false,
		},
		{
			Name:      "var-lib",
			MountPath: "/var/lib/openvswitch",
			ReadOnly:  false,
		},
	}
}

// GetVswitchdVolumeMounts - ovs-vswitchd VolumeMounts
func GetVswitchdVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "etc-machine-id",
			MountPath: "/etc/machine-id",
			ReadOnly:  true,
		},
		{
			Name:      "etc-localtime",
			MountPath: "/etc/localtime",
			ReadOnly:  true,
		},
		{
			Name:      "var-run",
			MountPath: "/var/run/openvswitch",
			ReadOnly:  false,
		},
		{
			Name:      "var-log",
			MountPath: "/var/log/openvswitch",
			ReadOnly:  false,
		},
		{
			Name:      "var-lib",
			MountPath: "/var/lib/openvswitch",
			ReadOnly:  false,
		},
	}

}
