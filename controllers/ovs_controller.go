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

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/daemonset"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/labels"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	ovsv1beta1 "github.com/openstack-k8s-operators/ovs-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/ovs-operator/pkg/ovs"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// OVSReconciler reconciles a OVS object
type OVSReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Log     logr.Logger
	Scheme  *runtime.Scheme
}

// GetClient -
func (r *OVSReconciler) GetClient() client.Client {
	return r.Client
}

// GetLogger -
func (r *OVSReconciler) GetLogger() logr.Logger {
	return r.Log
}

// +kubebuilder:rbac:groups=ovs.openstack.org,resources=ovs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ovs.openstack.org,resources=ovs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ovs.openstack.org,resources=ovs/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=*,verbs=*;
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=create;delete;get;list;patch;update;watch

// Reconcile - OVS
func (r *OVSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//_ = context.Background()
	_ = r.Log.WithValues("ovs", req.NamespacedName)

	// Fetch ovs instance
	instance := &ovsv1beta1.OVS{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	//
	// initialize status
	//
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
		// initialize conditions used later as Status=Unknown
		cl := condition.CreateList(
			condition.UnknownCondition(condition.DBReadyCondition, condition.InitReason, condition.DBReadyInitMessage),
			condition.UnknownCondition(condition.ExposeServiceReadyCondition, condition.InitReason, condition.ExposeServiceReadyInitMessage),
			condition.UnknownCondition(condition.InputReadyCondition, condition.InitReason, condition.InputReadyInitMessage),
			condition.UnknownCondition(condition.ServiceConfigReadyCondition, condition.InitReason, condition.ServiceConfigReadyInitMessage),
			condition.UnknownCondition(condition.DeploymentReadyCondition, condition.InitReason, condition.DeploymentReadyInitMessage),
		)

		instance.Status.Conditions.Init(&cl)

		// Register overall status immediately to have an early feedback e.g. in the cli
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		r.Log,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		// update the overall status condition if service is ready
		if instance.IsReady() {
			instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
		}

		if err := helper.SetAfter(instance); err != nil {
			util.LogErrorForObject(helper, err, "Set after and calc patch/diff", instance)
		}

		if changed := helper.GetChanges()["status"]; changed {
			patch := client.MergeFrom(helper.GetBeforeObject())

			if err := r.Status().Patch(ctx, instance, patch); err != nil && !k8s_errors.IsNotFound(err) {
				util.LogErrorForObject(helper, err, "Update status", instance)
			}
		}
	}()

	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper)
}

// SetupWithManager sets up the controller with the Manager.
func (r *OVSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ovsv1beta1.OVS{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&netattdefv1.NetworkAttachmentDefinition{}).
		Owns(&appsv1.DaemonSet{}).
		Complete(r)
}

func (r *OVSReconciler) reconcileDelete(ctx context.Context, instance *ovsv1beta1.OVS, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service delete")

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	r.Log.Info("Reconciled Service delete successfully")
	if err := r.Update(ctx, instance); err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *OVSReconciler) reconcileInit(
	ctx context.Context,
	instance *ovsv1beta1.OVS,
	helper *helper.Helper,
) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service init")

	//TODO(slaweq):
	// * read status of the external IDs
	// * if external IDs are different than required once, change them
	r.Log.Info("Reconciled Service init successfully")
	return ctrl.Result{}, nil
}

func (r *OVSReconciler) reconcileUpdate(ctx context.Context, instance *ovsv1beta1.OVS, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service update")

	r.Log.Info("Reconciled Service update successfully")
	return ctrl.Result{}, nil
}

func (r *OVSReconciler) reconcileUpgrade(ctx context.Context, instance *ovsv1beta1.OVS, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service upgrade")

	r.Log.Info("Reconciled Service upgrade successfully")
	return ctrl.Result{}, nil
}

func (r *OVSReconciler) reconcileNormal(ctx context.Context, instance *ovsv1beta1.OVS, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info("Reconciling Service")

	// If the service object doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(instance, helper.GetFinalizer())
	// Register the finalizer immediately to avoid orphaning resources on delete
	if err := r.Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}
	// ConfigMap
	configMapVars := make(map[string]env.Setter)

	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)

	//
	// create Configmap required for neutron input
	// - %-scripts configmap holding scripts to e.g. bootstrap the service
	//
	err := r.generateServiceConfigMaps(ctx, helper, instance, &configMapVars)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	//
	// create hash over all the different input resources to identify if any those changed
	// and a restart/recreate is required.
	//
	inputHash, err := r.createHashOfInputHashes(ctx, instance, configMapVars)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ServiceConfigReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceConfigReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	// TODO(slaweq): configure service (ovn controller and ovs settings)
	instance.Status.Conditions.MarkTrue(condition.ServiceConfigReadyCondition, condition.ServiceConfigReadyMessage)

	//
	// TODO check when/if Init, Update, or Upgrade should/could be skipped
	//

	serviceLabels := map[string]string{
		common.AppSelector: ovs.ServiceName,
	}

	// Handle service init
	ctrlResult, err := r.reconcileInit(ctx, instance, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	// Handle service update
	ctrlResult, err = r.reconcileUpdate(ctx, instance, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	// Handle service upgrade
	ctrlResult, err = r.reconcileUpgrade(ctx, instance, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	// Create additional Physical Network Attachements
	if err := ovs.CreateAdditionalNetworks(ctx, instance, serviceLabels, r.Client); err != nil {
		r.Log.Info(fmt.Sprintf("Failed to create additional networks: %s", err))
		return ctrlResult, err
	}

	// Define a new DaemonSet object
	ovsDaemonSet, err := ovs.DaemonSet(instance, inputHash, serviceLabels)
	if err != nil {
		r.Log.Error(err, "Failed to create OVS DaemonSet")
		return ctrl.Result{}, err
	}
	dset := daemonset.NewDaemonSet(
		ovsDaemonSet,
		5,
	)

	ctrlResult, err = dset.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DeploymentReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DeploymentReadyErrorMessage,
			err.Error()))
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.DeploymentReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DeploymentReadyRunningMessage))
		return ctrlResult, nil
	}

	instance.Status.DesiredNumberScheduled = dset.GetDaemonSet().Status.DesiredNumberScheduled
	instance.Status.NumberReady = dset.GetDaemonSet().Status.NumberReady

	if instance.IsReady() {
		instance.Status.Conditions.MarkTrue(condition.DeploymentReadyCondition, condition.DeploymentReadyMessage)
	}
	// create DaemonSet - end

	r.Log.Info("Reconciled Service successfully")

	return ctrl.Result{}, nil
}

//
// generateServiceConfigMaps - create create configmaps which hold scripts and service configuration
//
func (r *OVSReconciler) generateServiceConfigMaps(
	ctx context.Context,
	h *helper.Helper,
	instance *ovsv1beta1.OVS,
	envVars *map[string]env.Setter,
) error {
	// Create/update configmaps from templates
	cmLabels := labels.GetLabels(instance, labels.GetGroupLabel(ovs.ServiceName), map[string]string{})

	cms := []util.Template{
		// ScriptsConfigMap
		{
			Name:               fmt.Sprintf("%s-scripts", instance.Name),
			Namespace:          instance.Namespace,
			Type:               util.TemplateTypeScripts,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{"init.sh": "/bin/init.sh"},
			Labels:             cmLabels,
		},
	}
	err := configmap.EnsureConfigMaps(ctx, h, instance, cms, envVars)
	if err != nil {
		return nil
	}
	return nil
}

//
// createHashOfInputHashes - creates a hash of hashes which gets added to the resources which requires a restart
// if any of the input resources change, like configs, passwords, ...
//
func (r *OVSReconciler) createHashOfInputHashes(
	ctx context.Context,
	instance *ovsv1beta1.OVS,
	envVars map[string]env.Setter,
) (string, error) {
	mergedMapVars := env.MergeEnvs([]corev1.EnvVar{}, envVars)
	hash, err := util.ObjectHash(mergedMapVars)
	if err != nil {
		return hash, err
	}
	if hashMap, changed := util.SetHash(instance.Status.Hash, common.InputHashName, hash); changed {
		instance.Status.Hash = hashMap
		if err := r.Client.Status().Update(ctx, instance); err != nil {
			return hash, err
		}
		r.Log.Info(fmt.Sprintf("Input maps hash %s - %s", common.InputHashName, hash))
	}
	return hash, nil
}
