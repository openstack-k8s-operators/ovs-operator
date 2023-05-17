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

package functional_test

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	networkv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/openstack-k8s-operators/lib-common/modules/test/helpers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

var _ = Describe("OVS controller", func() {

	When("A OVS instance is created", func() {
		var OVSName types.NamespacedName
		BeforeEach(func() {
			name := fmt.Sprintf("ovs-%s", uuid.New().String())
			instance := CreateOVS(namespace, name, GetDefaultOVSSpec())
			OVSName = types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()}
			DeferCleanup(th.DeleteInstance, instance)
		})

		It("should have the Spec fields initialized", func() {
			OVS := GetOVS(OVSName)
			Expect(OVS.Spec.OvsContainerImage).Should(Equal("quay.io/podified-antelope-centos9/openstack-ovn-base:current-podified"))
			Expect(OVS.Spec.OvnContainerImage).Should(Equal("quay.io/podified-antelope-centos9/openstack-ovn-controller:current-podified"))
		})

		It("should have the Status fields initialized", func() {
			OVS := GetOVS(OVSName)
			Expect(OVS.Status.Hash).To(BeEmpty())
			Expect(OVS.Status.NumberReady).To(Equal(int32(0)))
		})

		It("should have a finalizer", func() {
			// the reconciler loop adds the finalizer so we have to wait for
			// it to run
			Eventually(func() []string {
				return GetOVS(OVSName).Finalizers
			}, timeout, interval).Should(ContainElement("OVS"))
		})

		It("should not create a config map", func() {
			Eventually(func() []corev1.ConfigMap {
				return th.ListConfigMaps(fmt.Sprintf("%s-%s", OVSName.Name, "scripts")).Items
			}, timeout, interval).Should(BeEmpty())
		})

		It("should be in input ready condition", func() {
			th.ExpectCondition(
				OVSName,
				ConditionGetterFunc(OVSConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionTrue,
			)
		})

		When("OVNDBCluster instances are available", func() {
			It("should create a ConfigMap for init.sh with the ovn remote config option set based on the OVNDBCluster", func() {
				dbs := CreateOVNDBClusters(namespace)
				DeferCleanup(DeleteOVNDBClusters, dbs)
				daemonSetName := types.NamespacedName{
					Namespace: namespace,
					Name:      "ovs",
				}
				SimulateDaemonsetNumberReady(daemonSetName)
				scriptsCM := types.NamespacedName{
					Namespace: OVSName.Namespace,
					Name:      fmt.Sprintf("%s-%s", OVSName.Name, "scripts"),
				}

				Eventually(func() corev1.ConfigMap {
					return *th.GetConfigMap(scriptsCM)
				}, timeout, interval).ShouldNot(BeNil())
				for _, db := range dbs {
					ovndb := GetOVNDBCluster(db)
					Expect(th.GetConfigMap(scriptsCM).Data["init.sh"]).Should(
						ContainSubstring("ovn-remote=%s", ovndb.Status.DBAddress))
				}

				th.ExpectCondition(
					OVSName,
					ConditionGetterFunc(OVSConditionGetter),
					condition.ServiceConfigReadyCondition,
					corev1.ConditionTrue,
				)
			})
		})

		When("OVS CR is deleted", func() {
			It("removes the Config MAP", func() {
				DeferCleanup(DeleteOVNDBClusters, CreateOVNDBClusters(namespace))
				scriptsCM := types.NamespacedName{
					Namespace: OVSName.Namespace,
					Name:      fmt.Sprintf("%s-%s", OVSName.Name, "scripts"),
				}

				Eventually(func() corev1.ConfigMap {
					return *th.GetConfigMap(scriptsCM)
				}, timeout, interval).ShouldNot(BeNil())

				th.DeleteInstance(GetOVS(OVSName))

				Eventually(func() []corev1.ConfigMap {
					return th.ListConfigMaps(scriptsCM.Name).Items
				}, timeout, interval).Should(BeEmpty())
			})
		})

	})

	When("OVS is created with networkAttachments", func() {
		var OVSName types.NamespacedName
		BeforeEach(func() {
			dbs := CreateOVNDBClusters(namespace)
			DeferCleanup(DeleteOVNDBClusters, dbs)
			name := fmt.Sprintf("ovs-%s", uuid.New().String())
			spec := GetDefaultOVSSpec()
			spec["networkAttachment"] = "internalapi"
			instance := CreateOVS(namespace, name, spec)
			OVSName = types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()}
			DeferCleanup(th.DeleteInstance, instance)
		})

		It("reports that the definition is missing", func() {
			th.ExpectConditionWithDetails(
				OVSName,
				ConditionGetterFunc(OVSConditionGetter),
				condition.NetworkAttachmentsReadyCondition,
				corev1.ConditionFalse,
				condition.RequestedReason,
				"NetworkAttachment resources missing: internalapi",
			)
		})
		It("reports that network attachment is missing", func() {
			internalAPINADName := types.NamespacedName{Namespace: namespace, Name: "internalapi"}
			nad := th.CreateNetworkAttachmentDefinition(internalAPINADName)
			DeferCleanup(th.DeleteInstance, nad)

			daemonSetName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovs",
			}
			ds := GetDaemonSet(daemonSetName)

			expectedAnnotation, err := json.Marshal(
				[]networkv1.NetworkSelectionElement{
					{
						Name:             "internalapi",
						Namespace:        namespace,
						InterfaceRequest: "internalapi",
					},
				})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ds.Spec.Template.ObjectMeta.Annotations).To(
				HaveKeyWithValue("k8s.v1.cni.cncf.io/networks", string(expectedAnnotation)),
			)

			// We don't add network attachment status annotations to the Pods
			// to simulate that the network attachments are missing.
			//SimulateDaemonsetNumberReadyWithPods(daemonSetName, map[string][]string{})

			th.ExpectConditionWithDetails(
				OVSName,
				ConditionGetterFunc(OVSConditionGetter),
				condition.NetworkAttachmentsReadyCondition,
				corev1.ConditionFalse,
				condition.ErrorReason,
				"NetworkAttachments error occurred "+
					"not all pods have interfaces with ips as configured in NetworkAttachments: internalapi",
			)
		})
		It("reports that an IP is missing", func() {
			internalAPINADName := types.NamespacedName{Namespace: namespace, Name: "internalapi"}
			nad := th.CreateNetworkAttachmentDefinition(internalAPINADName)
			DeferCleanup(th.DeleteInstance, nad)

			daemonSetName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovs",
			}
			ds := GetDaemonSet(daemonSetName)

			expectedAnnotation, err := json.Marshal(
				[]networkv1.NetworkSelectionElement{
					{
						Name:             "internalapi",
						Namespace:        namespace,
						InterfaceRequest: "internalapi",
					},
				})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ds.Spec.Template.ObjectMeta.Annotations).To(
				HaveKeyWithValue("k8s.v1.cni.cncf.io/networks", string(expectedAnnotation)),
			)

			// We simulate that there is no IP associated with the internalapi
			// network attachment
			SimulateDaemonsetNumberReadyWithPods(
				daemonSetName,
				map[string][]string{namespace + "/internalapi": {}},
			)

			th.ExpectConditionWithDetails(
				OVSName,
				ConditionGetterFunc(OVSConditionGetter),
				condition.NetworkAttachmentsReadyCondition,
				corev1.ConditionFalse,
				condition.ErrorReason,
				"NetworkAttachments error occurred "+
					"not all pods have interfaces with ips as configured in NetworkAttachments: internalapi",
			)
		})
		It("reports NetworkAttachmentsReady if the Pods got the proper annotations", func() {
			internalAPINADName := types.NamespacedName{Namespace: namespace, Name: "internalapi"}
			nad := th.CreateNetworkAttachmentDefinition(internalAPINADName)
			DeferCleanup(th.DeleteInstance, nad)

			daemonSetName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovs",
			}
			SimulateDaemonsetNumberReadyWithPods(
				daemonSetName,
				map[string][]string{namespace + "/internalapi": {"10.0.0.1"}},
			)

			th.ExpectCondition(
				OVSName,
				ConditionGetterFunc(OVSConditionGetter),
				condition.NetworkAttachmentsReadyCondition,
				corev1.ConditionTrue,
			)

			Eventually(func(g Gomega) {
				OVS := GetOVS(OVSName)
				g.Expect(OVS.Status.NetworkAttachments).To(
					Equal(map[string][]string{namespace + "/internalapi": {"10.0.0.1"}}))

			}, timeout, interval).Should(Succeed())
		})
	})

	When("OVS is created with nic configs", func() {
		var OVSName types.NamespacedName
		BeforeEach(func() {
			dbs := CreateOVNDBClusters(namespace)
			DeferCleanup(DeleteOVNDBClusters, dbs)
			name := fmt.Sprintf("ovs-%s", uuid.New().String())
			spec := GetDefaultOVSSpec()
			spec["nicMappings"] = map[string]interface{}{
				"physnet1": "enp2s0.100",
			}
			instance := CreateOVS(namespace, name, spec)
			OVSName = types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()}
			DeferCleanup(th.DeleteInstance, instance)
		})

		It("reports that the networkattachment definition is created based on nic configs", func() {
			daemonSetName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovs",
			}
			ds := GetDaemonSet(daemonSetName)

			expectedAnnotation, err := json.Marshal(
				[]networkv1.NetworkSelectionElement{
					{
						Name:             "physnet1",
						Namespace:        namespace,
						InterfaceRequest: "physnet1",
					},
				})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ds.Spec.Template.ObjectMeta.Annotations).To(
				HaveKeyWithValue("k8s.v1.cni.cncf.io/networks", string(expectedAnnotation)),
			)

			th.ExpectCondition(
				OVSName,
				ConditionGetterFunc(OVSConditionGetter),
				condition.NetworkAttachmentsReadyCondition,
				corev1.ConditionTrue,
			)
		})

		It("reports IP to not exist in Status for nic-configs", func() {
			daemonSetName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovs",
			}
			SimulateDaemonsetNumberReadyWithPods(
				daemonSetName,
				map[string][]string{namespace + "/physnet1": {"10.0.0.1"}},
			)

			th.ExpectCondition(
				OVSName,
				ConditionGetterFunc(OVSConditionGetter),
				condition.NetworkAttachmentsReadyCondition,
				corev1.ConditionTrue,
			)

			Eventually(func(g Gomega) {
				OVS := GetOVS(OVSName)
				g.Expect(OVS.Status.NetworkAttachments).ToNot(
					Equal(map[string][]string{namespace + "/physnet1": {"10.0.0.1"}}))

			}, timeout, interval).Should(Succeed())
		})
	})

	When("OVS is created with networkAttachment and nic configs", func() {
		BeforeEach(func() {
			dbs := CreateOVNDBClusters(namespace)
			DeferCleanup(DeleteOVNDBClusters, dbs)
			name := fmt.Sprintf("ovs-%s", uuid.New().String())
			spec := GetDefaultOVSSpec()
			spec["networkAttachment"] = "internalapi"
			spec["nicMappings"] = map[string]interface{}{
				"physnet1": "enp2s0.100",
			}
			instance := CreateOVS(namespace, name, spec)
			DeferCleanup(th.DeleteInstance, instance)
		})

		It("reports that daemonset have annotations for both Networkattachment and nic-configs", func() {
			internalAPINADName := types.NamespacedName{Namespace: namespace, Name: "internalapi"}
			nad := th.CreateNetworkAttachmentDefinition(internalAPINADName)
			DeferCleanup(th.DeleteInstance, nad)

			daemonSetName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovs",
			}
			ds := GetDaemonSet(daemonSetName)

			expectedAnnotation, err := json.Marshal(
				[]networkv1.NetworkSelectionElement{
					{
						Name:             "internalapi",
						Namespace:        namespace,
						InterfaceRequest: "internalapi",
					},
					{
						Name:             "physnet1",
						Namespace:        namespace,
						InterfaceRequest: "physnet1",
					},
				})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ds.Spec.Template.ObjectMeta.Annotations).To(
				HaveKeyWithValue("k8s.v1.cni.cncf.io/networks", string(expectedAnnotation)),
			)
		})
	})

})