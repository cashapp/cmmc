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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cmmcv1beta1 "github.com/cashapp/cmmc/api/v1beta1"
	"github.com/cashapp/cmmc/util"
	anns "github.com/cashapp/cmmc/util/annotations"
	"github.com/cashapp/cmmc/util/finalizer"
	"github.com/cashapp/cmmc/util/metrics"
	"github.com/pkg/errors"
)

// MergeSourceReconciler reconciles a MergeSource object.
type MergeSourceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder *metrics.Recorder
}

//+kubebuilder:rbac:groups=config.cmmc.k8s.cash.app,resources=mergesources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=config.cmmc.k8s.cash.app,resources=mergesources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=config.cmmc.k8s.cash.app,resources=mergesources/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MergeSourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// check to see if we're getting a configMap from our watcher
	watched := r.watchedConfigMap(ctx, req.NamespacedName)
	if watched == nil {
		// received ConfigMap that isn't watched / annotated, so we are skipping
		// work here, nothing else to do, nothing to resubmit.
		log.Info("irrelevant configmap, skipping reconcile")
		return ctrl.Result{}, nil
	}

	mergeSource, err := r.mergeSource(ctx, watched)
	if err != nil {
		log.Error(err, "failed to fetch merge source")
		return ctrl.Result{Requeue: true}, err
	}

	stop, err := r.reconcileMergeSource(ctx, mergeSource, watched)
	if err != nil {
		return ctrl.Result{Requeue: true}, errors.Wrap(err, "could not reconcile MergeSource")
	} else if stop {
		return ctrl.Result{}, nil
	}

	if err := r.maybeRemoveWatchedAnnotation(ctx, watched); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// means we were coming from a ConfigMap
	if watched.WatchedBy != req.NamespacedName {
		return ctrl.Result{}, nil
	}

	// Actually were reconciling & managing our MergeSource,
	// we are going to check and see if there are going to be any
	// other sources existing...
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

func (r *MergeSourceReconciler) reconcileMergeSource(
	ctx context.Context, mergeSource *MergeSource, watched *watchedConfigMap,
) (bool, error) {
	if mergeSource == nil {
		return false, nil
	}

	log := log.FromContext(ctx)

	isDeleting, err := finalizer.
		New(
			mergeSourceFinalizerName,
			func() error {
				return r.finalizeDeletion(ctx, mergeSource, watched)
			},
			func() error {
				r.Recorder.RecordNumSources(mergeSource, 0)
				r.Recorder.RecordCondition(mergeSource, metav1.Condition{Type: "Ready"})
				return nil
			},
		).
		Execute(ctx, r.Client, mergeSource)
	if err != nil {
		return false, errors.WithStack(err)
	} else if isDeleting {
		return true, nil
	}

	defer r.Recorder.RecordReadyCondition(mergeSource)

	sources, err := r.sources(ctx, mergeSource)
	if err != nil {
		return false, errors.WithStack(client.IgnoreNotFound(err))
	}

	r.Recorder.RecordNumSources(mergeSource, len(sources))
	log = log.WithValues("numSources", len(sources))

	var output string
	for _, cm := range sources {
		data, err := r.configMapOutput(ctx, cm, mergeSource.Spec.Source.Data, watched)
		if err != nil {
			return false, errors.Wrap(err, "failed accumulating source")
		}
		output += data
	}

	// Retrieve new copy of the current MergeSource so that we're updating the most recent
	// version of the resource when we update its status.
	ms := &cmmcv1beta1.MergeSource{}
	err = r.Get(ctx, types.NamespacedName{
		Namespace: mergeSource.Namespace,
		Name:      mergeSource.Name,
	}, ms)
	if err != nil {
		return false, errors.Wrapf(err, "error retrieving mergeSource %s during status update phase", mergeSource.Name)
	}

	// Use the newly retrieved MergeSource to update the status.
	ms.Status.Output = output
	ms.SetStatusCondition(cmmcv1beta1.MergeSourceConditionReady(len(sources)))
	if err = r.Status().Update(ctx, ms); err != nil {
		return false, errors.Wrap(err, "failed updating status after accumulating watched resources")
	}

	log.Info("updated status")
	return false, nil
}

func (r *MergeSourceReconciler) finalizeDeletion(
	ctx context.Context, s *MergeSource, w *watchedConfigMap,
) error {
	sources, err := r.sources(ctx, s)
	if err != nil {
		return err
	}

	for _, cm := range sources {
		cm := cm
		if err := r.cleanUpWatchedByAnnotation(ctx, &cm, w.WatchedBy.String()); err != nil {
			return err
		}
	}

	r.Recorder.RecordNumSources(s, 0)

	return nil
}

func (r *MergeSourceReconciler) mergeSource(
	ctx context.Context, w *watchedConfigMap,
) (*MergeSource, error) {
	var s cmmcv1beta1.MergeSource
	err := r.Get(ctx, w.WatchedBy, &s)
	if err == nil {
		return &s, nil
	}

	if !apierrors.IsNotFound(err) {
		return nil, errors.WithStack(err)
	}

	return nil, nil //nolint:nilnil
}

func (r *MergeSourceReconciler) sources(
	ctx context.Context, s *cmmcv1beta1.MergeSource,
) ([]corev1.ConfigMap, error) {
	var sources corev1.ConfigMapList
	if err := r.List(ctx, &sources, client.MatchingLabels(s.Spec.Selector)); err != nil {
		return nil, errors.WithStack(err)
	}

	if len(sources.Items) == 0 {
		return nil, nil
	}

	namespaceSelector := s.NamespaceSelector()
	if len(namespaceSelector) == 0 {
		return sources.Items, nil
	}

	var nsList corev1.NamespaceList
	if err := r.List(ctx, &nsList, client.MatchingLabels(namespaceSelector)); err != nil {
		return nil, errors.WithStack(err)
	}

	if len(nsList.Items) == 0 {
		log.FromContext(ctx).Info("[WARN] found no matching namespaces, filtering all configMaps", "selector", namespaceSelector)
		return nil, nil
	}

	nsMap := map[string]struct{}{}
	for _, ns := range nsList.Items {
		nsMap[ns.GetName()] = struct{}{}
	}

	n := 0
	for _, cm := range sources.Items {
		_, ok := nsMap[cm.GetNamespace()]
		if ok {
			sources.Items[n] = cm
			n++
		}
	}

	return sources.Items[:n], nil
}

func (r *MergeSourceReconciler) configMapOutput(
	ctx context.Context, cm corev1.ConfigMap,
	key string,
	w *watchedConfigMap,
) (string, error) {
	if w.Name == util.ObjectResourceName(&cm) {
		w.ShouldBeRemoved = false
	}

	// ensure selector matched configMap has the watchedByAnnotation
	if err := r.annotateWatchedByConfigMap(ctx, &cm, w.WatchedBy.String()); err != nil {
		return "", errors.Wrap(err, "error updating watchedBy annotation on configMap")
	}

	data := cm.Data[key]
	return data, nil
}

// Delete the annotation if there is no merge source watching the configmap
// Otherwise, remove the mergesource from the annotations.
func (r *MergeSourceReconciler) cleanUpWatchedByAnnotation(
	ctx context.Context, cm *corev1.ConfigMap, name string,
) error {
	return errors.WithStack(anns.Apply(ctx, r.Client, cm, watchedBy.RemoveFromList(name)))
}

func (r *MergeSourceReconciler) annotateWatchedByConfigMap(
	ctx context.Context, cm *corev1.ConfigMap, name string,
) error {
	// Retrieve the latest copy of the configmap we're looking to update
	// before triggering the update to prevent errors with updating a resource
	// without the most recent changes
	currentCm := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: cm.Namespace,
		Name:      cm.Name,
	}, currentCm)
	if err != nil {
		// If not found, the ConfigMap has been deleted
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "error retrieving configmap %s for watched annotation update", cm.Name)
	}

	return errors.WithStack(anns.Apply(ctx, r.Client, currentCm, watchedBy.AddToList(name)))
}

type watchedConfigMap struct {
	Resource        corev1.ConfigMap
	Name            string
	ShouldBeRemoved bool
	WatchedBy       types.NamespacedName
}

func (r *MergeSourceReconciler) watchedConfigMap(
	ctx context.Context, name types.NamespacedName,
) *watchedConfigMap {
	var cm corev1.ConfigMap
	if err := r.Get(ctx, name, &cm); err != nil {
		// If we didn't find a configMap, we have nothing to remove, and we
		// can keep going to check on the name being a MergeSource.
		return &watchedConfigMap{WatchedBy: name}
	}

	// if it's not being watched by anything, we can abort here
	// since we found an irrelevant configMap
	n, ok := watchedBy.ParseObjectName(&cm)
	if !ok {
		return nil
	}

	return &watchedConfigMap{
		ShouldBeRemoved: true, // assume we should be removing it unless proven otherwise
		Resource:        cm,
		Name:            util.ObjectResourceName(&cm),
		WatchedBy:       n,
	}
}

func (r *MergeSourceReconciler) maybeRemoveWatchedAnnotation(
	ctx context.Context, w *watchedConfigMap,
) error {
	if !w.ShouldBeRemoved {
		return nil
	}

	log.FromContext(ctx).Info("attempting to remove annotation", "config-map", w.Name)
	err := r.cleanUpWatchedByAnnotation(ctx, &w.Resource, w.WatchedBy.String())
	if err != nil {
		return err
	}

	log.FromContext(ctx).Info("cleaned up annotation")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MergeSourceReconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return errors.WithStack(
		ctrl.NewControllerManagedBy(mgr).
			For(&cmmcv1beta1.MergeSource{}).
			WithOptions(opts).
			Watches(
				&source.Kind{Type: &corev1.ConfigMap{}},
				watchReconciliationEventHandler(
					watchedBy.ParseObjectName,
					func(o client.Object) (types.NamespacedName, bool) {
						n := util.ObjectNamespacedName(o)
						return n, true
					},
				),
			).Complete(r),
	)
}
