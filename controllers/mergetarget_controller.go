/*
Copyright 2021 Square, Inc

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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	cmmcv1beta1 "github.com/cashapp/cmmc/api/v1beta1"
	anns "github.com/cashapp/cmmc/util/annotations"
	"github.com/cashapp/cmmc/util/finalizer"
	"github.com/cashapp/cmmc/util/metrics"
	corev1 "k8s.io/api/core/v1"
)

// MergeTargetReconciler reconciles a MergeTarget object.
type MergeTargetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder *metrics.Recorder
}

//+kubebuilder:rbac:groups=config.cmmc.k8s.cash.app,resources=mergetargets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=config.cmmc.k8s.cash.app,resources=mergetargets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=config.cmmc.k8s.cash.app,resources=mergetargets/finalizers,verbs=update
//+kubebuilder:rbac:groups=config.cmmc.k8s.cash.app,resources=mergesources,verbs=get;list;watch
//+kubebuilder:rbac:groups=config.cmmc.k8s.cash.app,resources=mergesources/status,verbs=get;list
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;update;create;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MergeTargetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		mtName = req.NamespacedName.String()
		log    = log.FromContext(ctx)
	)

	// 1. We fetch the MergeTarget.
	var mergeTarget cmmcv1beta1.MergeTarget
	err := r.Get(ctx, req.NamespacedName, &mergeTarget)
	if err != nil {
		return ctrl.Result{}, errors.WithStack(client.IgnoreNotFound(err))
	}

	setStatusCondition := func(c metav1.Condition) error {
		return errors.WithStack(r.setStatusCondition(ctx, &mergeTarget, c))
	}

	// 2. Validate the spec.target name, (the ConfigMap), and update
	//    the status accordingly if it's bad.
	targetName, err := mergeTarget.NamespacedTargetName()
	if err != nil {
		_ = setStatusCondition(cmmcv1beta1.MergeTargetConditionMissingTarget(err))
		return ctrl.Result{RequeueAfter: time.Minute}, errors.WithStack(err)
	}

	// 3. Ensure finalizer / cleanup is running correctly.
	isDeleting, err := finalizer.
		New(
			mergeTargetFinalizerName,
			func() error {
				return r.finalizeDeletion(ctx, targetName, &mergeTarget)
			},
			func() error {
				r.Recorder.RecordNumSources(&mergeTarget, 0)
				r.Recorder.RecordCondition(&mergeTarget, metav1.Condition{Type: "Ready"})
				return nil
			},
		).
		Execute(ctx, r.Client, &mergeTarget)
	if err != nil {
		return ctrl.Result{Requeue: true}, errors.WithStack(err)
	} else if isDeleting {
		return ctrl.Result{}, nil
	}

	defer r.Recorder.RecordReadyCondition(&mergeTarget)

	// 4. Find/setup the target ConfigMap
	targetConfigMap, requeue, err := r.targetConfigMap(ctx, &mergeTarget, targetName, mtName)
	if err != nil {
		log.Info("error fetching target config-map")
		return ctrl.Result{Requeue: requeue}, err
	}

	// 5. Do actual recondiliation.
	result, err := r.reconcileMergeTarget(ctx, mtName, &mergeTarget, targetConfigMap)
	return result, errors.WithStack(err)
}

// reconcileMergeTarget is the main function that ensures the target ConfigMap
// has the state it needs to have given the MergeTarget resource.
func (r *MergeTargetReconciler) reconcileMergeTarget(ctx context.Context, mtName string, mt *MergeTarget, cm *corev1.ConfigMap) (ctrl.Result, error) {
	var (
		log                = log.FromContext(ctx)
		setStatusCondition = func(c metav1.Condition) error {
			return errors.WithStack(r.setStatusCondition(ctx, mt, c))
		}
	)

	if err := r.updateDataStatus(ctx, mt, cm); err != nil {
		return ctrl.Result{Requeue: true}, errors.Wrap(err, "failed updating MergeTarget Status")
	}

	log.Info("initialized/updated initial state of the target")

	stats, err := r.reduceTargetState(ctx, mtName, mt, cm)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, errors.WithStack(err)
	}

	r.Recorder.RecordNumSources(mt, stats.NumMergeSources)
	stats.LogWithValues(log).Info("found and merged sources")
	err = setStatusCondition(cmmcv1beta1.MergeTargetConditionValidation(stats.FieldsErrorMsgs, stats.NumMergeSources))
	if err != nil {
		return ctrl.Result{Requeue: true}, errors.WithStack(err)
	}

	if stats.NumUpdatedKeys > 0 {
		if err := r.Update(ctx, cm); err != nil {
			_ = setStatusCondition(cmmcv1beta1.MergeTargetConditionErrorUpdating(err, stats.NumUpdatedKeys))
			return ctrl.Result{RequeueAfter: time.Minute}, errors.Wrap(err, "failed updating target configMap")
		}
	}

	err = setStatusCondition(cmmcv1beta1.MergeTargetConditionReady(len(stats.FieldsErrorMsgs) > 0))
	return ctrl.Result{RequeueAfter: time.Minute}, errors.WithStack(err)
}

type mergeStats struct {
	NumUpdatedKeys  int
	NumMergeSources int
	FieldsErrorMsgs []string
}

func (m *mergeStats) LogWithValues(l logr.Logger) logr.Logger {
	return l.WithValues(
		"errorsOnFields", len(m.FieldsErrorMsgs),
		"numUpdatedKeys", m.NumUpdatedKeys,
		"numMergeSources", m.NumMergeSources,
	)
}

func (r *MergeTargetReconciler) updateDataStatus(ctx context.Context, mt *MergeTarget, cm *corev1.ConfigMap) error {
	mt.UpdateDataStatus(cm.Data)
	return errors.Wrap(r.Status().Update(ctx, mt), "failed updating initial status")
}

func (r *MergeTargetReconciler) setStatusCondition(ctx context.Context, mt *MergeTarget, c metav1.Condition) error {
	mt.SetStatusCondition(c)
	return errors.Wrapf(r.Status().Update(ctx, mt), "error updating condition type %s", c.Type)
}

func (r *MergeTargetReconciler) reduceTargetState(
	ctx context.Context,
	name string,
	mergeTarget *cmmcv1beta1.MergeTarget,
	targetConfigMap *corev1.ConfigMap,
) (*mergeStats, error) {
	var mergeSources cmmcv1beta1.MergeSourceList
	if err := r.List(ctx, &mergeSources, client.MatchingFields{"status.target": name}); err != nil {
		return nil, errors.Wrapf(err, "failed fetching MergeSource list for %s", name)
	}

	numUpdatedKeys, errorsOnFields := mergeTarget.ReduceDataState(mergeSources, &targetConfigMap.Data)
	return &mergeStats{
		NumUpdatedKeys:  numUpdatedKeys,
		NumMergeSources: len(mergeSources.Items),
		FieldsErrorMsgs: errorsOnFields,
	}, nil
}

func (r *MergeTargetReconciler) targetConfigMap(
	ctx context.Context,
	mergeTarget *cmmcv1beta1.MergeTarget,
	name types.NamespacedName,
	managedByName string,
) (*corev1.ConfigMap, bool, error) {
	var cm corev1.ConfigMap
	if err := r.Get(ctx, name, &cm); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, errors.Wrap(err, "error fetching lonfigMap")
		}

		if err := r.maybeSetNewlyCreated(
			ctx,
			mergeTarget,
			cmmcv1beta1.DataNewlyCreatedStatusYes,
		); err != nil {
			return nil, true, errors.WithStack(err)
		}

		cm = emptyConfigMap(name)
		if err := r.Create(ctx, &cm); err != nil {
			return nil, true, errors.Wrap(err, "failed to create target configMap")
		}

		log.FromContext(ctx).Info("created target-cm", "name", name.String())
	}

	if err := r.maybeSetNewlyCreated(
		ctx,
		mergeTarget,
		cmmcv1beta1.DataNewlyCreatedStatusNo,
	); err != nil {
		return nil, true, err
	}

	requeue, err := r.ensureManagedByAnnotation(ctx, &cm, managedByName)
	if err != nil {
		return nil, requeue, err
	}

	return &cm, false, nil
}

var errMisconfiguredTargetConfigMap = errors.New("misconfigured target ConfigMap")

func (r *MergeTargetReconciler) ensureManagedByAnnotation(
	ctx context.Context, cm *corev1.ConfigMap, managedByName string,
) (bool, error) {
	managedBy, exists := managedByMergeTarget.ParseObjectName(cm)
	if !exists {
		err := anns.Apply(ctx, r.Client, cm, managedByMergeTarget.Add(managedByName))
		return true, errors.WithStack(err)
	}

	if managedBy.String() != managedByName {
		return false, fmt.Errorf("cm managed by %s: %w", managedBy, errMisconfiguredTargetConfigMap)
	}

	return false, nil
}

func (r *MergeTargetReconciler) maybeSetNewlyCreated(
	ctx context.Context, t *MergeTarget, to string,
) error {
	if t.Status.NewlyCreated != "" {
		return nil
	}

	t.Status.NewlyCreated = to
	return errors.WithStack(r.Status().Update(ctx, t))
}

func (r *MergeTargetReconciler) finalizeDeletion(
	ctx context.Context, name types.NamespacedName, t *MergeTarget,
) error {
	var cm corev1.ConfigMap
	if err := r.Get(ctx, name, &cm); err != nil {
		// if the CM doesn't exist we are probably done
		// there might be some weird issue where it doesn't exist
		// and it _should_-- while we are deleting the MergeTarget
		// and that entire thing is a pretty wild potential race condition.
		return errors.Wrap(client.IgnoreNotFound(err), "error fetching target configMap during deletion")
	}

	if t.IsStatusNewlyCreated() {
		// we need to do some cleanup to this configMap, which exists
		// simplest case is that we should be deleting this.
		return errors.Wrap(r.Delete(ctx, &cm), "error deleting target configMap")
	}

	// otherwise we have to clean up all the fields!
	for k, v := range t.Status.Data {
		if v.IsStatusNewlyCreated() {
			delete(cm.Data, k)
		} else {
			cm.Data[k] = v.Init
		}
	}

	// remove the annotation
	anns.Set(&cm, managedByMergeTarget.Remove())

	// perform the udpate
	return errors.Wrapf(r.Update(ctx, &cm), "error reverting fields of target configMap %s", name)
}

const (
	fieldIndexStatusTarget = "status.target"
)

func resourceStatusTargetIndexer(o client.Object) []string {
	target, ok := cmmcv1beta1.MergeSourceNamespacedTargetName(o)
	if !ok {
		return nil
	}
	return []string{target.String()}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MergeTargetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	if err := mgr.GetFieldIndexer().IndexField(
		ctx, &cmmcv1beta1.MergeSource{}, fieldIndexStatusTarget, resourceStatusTargetIndexer,
	); err != nil {
		return errors.Wrapf(err, "error setting field indexer for field = %s", fieldIndexStatusTarget)
	}

	return errors.WithStack(
		ctrl.NewControllerManagedBy(mgr).
			For(&cmmcv1beta1.MergeTarget{}).
			Watches(
				&source.Kind{Type: &corev1.ConfigMap{}},
				watchReconciliationEventHandler(managedByMergeTarget.ParseObjectName),
			).
			Watches(
				&source.Kind{Type: &cmmcv1beta1.MergeSource{}},
				watchReconciliationEventHandler(cmmcv1beta1.MergeSourceNamespacedTargetName),
			).
			Complete(r),
	)
}

func emptyConfigMap(name types.NamespacedName) corev1.ConfigMap {
	return corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Data: nil,
	}
}
