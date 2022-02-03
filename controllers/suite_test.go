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
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	cmmcv1beta1 "github.com/cashapp/cmmc/api/v1beta1"
	"github.com/cashapp/cmmc/util/metrics"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	var err error
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = cmmcv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = cmmcv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())

	recorder := metrics.NewRecorder()

	err = (&MergeTargetReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: recorder,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&MergeSourceReconciler{
		Client:   k8sManager.GetClient(),
		Scheme:   k8sManager.GetScheme(),
		Recorder: recorder,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()
}, 60)

var _ = Describe("cmmc", func() {

	defer GinkgoRecover()

	BeforeEach(func() {
		Expect(k8sClient).ToNot(BeNil())
	})

	const (
		// timeout is how long we wait for our `Eventually` calls to finish
		// since we are running in a test k8s cluster, the faster/slower the
		// computer we are running these tests on the shorter/longer this timeout
		// needs to be to detect failing cases.
		timeout = time.Second * 10
		// interval is how often we poll each `Eventually` call.
		interval = time.Millisecond * 250
	)

	var (
		rolesMergeSource *cmmcv1beta1.MergeSource
		usersMergeSource *cmmcv1beta1.MergeSource

		ctx    = context.Background()
		cmName = types.NamespacedName{Namespace: "default", Name: "test-cm-1"}

		// selector is the standard label/selector we are going to be
		// using for our tests to watch ConfigMaps
		selector = map[string]string{
			"test-label": "for-this-source",
		}
	)

	It("should create a ConfigMap", func() {
		cm := &corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{Name: cmName.Name, Namespace: cmName.Namespace, Labels: selector},
			Data:       map[string]string{"mapRoles": mapRoles1, "mapUsers": mapUsers1},
		}
		Expect(k8sClient.Create(ctx, cm)).Should(Succeed())
	})

	When("We create MergeSources", func() {
		rolesMergeSource = cmmcv1beta1.NewMergeSource("default", "map-roles-source", cmmcv1beta1.MergeSourceSpec{
			Selector: selector,
			Source:   cmmcv1beta1.MergeSourceSourceSpec{Data: "mapRoles"},
			Target:   cmmcv1beta1.MergeSourceTargetSpec{Name: "aws-auth-target", Data: "mapRoles"},
		})
		Expect(k8sClient.Create(ctx, rolesMergeSource)).Should(Succeed())

		usersMergeSource = cmmcv1beta1.NewMergeSource("default", "map-users-source", cmmcv1beta1.MergeSourceSpec{
			Selector: selector,
			Source:   cmmcv1beta1.MergeSourceSourceSpec{Data: "mapUsers"},
			Target:   cmmcv1beta1.MergeSourceTargetSpec{Name: "aws-auth-target", Data: "mapUsers"},
		})
		Expect(k8sClient.Create(ctx, usersMergeSource)).Should(Succeed())

		It("should annotate the created ConfigMap", func() {
			Eventually(func() (string, error) {
				cm := &corev1.ConfigMap{}
				if err := k8sClient.Get(ctx, cmName, cm); err != nil {
					return "", err
				}
				return cm.GetAnnotations()[watchedByAnnotation], nil
			}, timeout, interval).Should(Equal("default/aws-auth-map-roles-source,default/aws-auth-map-users-source"))
		})
	})

	When("we create a MergeTarget", func() {
		mergeTargetName := types.NamespacedName{Namespace: "default", Name: "merge-me"}
		target := cmmcv1beta1.NewMergeTarget("default", "target", cmmcv1beta1.MergeTargetSpec{
			Target: mergeTargetName.String(),
			Data: map[string]cmmcv1beta1.MergeTargetDataSpec{
				"mapRoles": {},
				"mapUsers": {},
			},
		})
		Expect(k8sClient.Create(ctx, target)).Should(Succeed())

		It("should create a target ConfigMap", func() {
			Eventually(func() (*configMapState, error) {
				var cm corev1.ConfigMap
				if err := k8sClient.Get(ctx, mergeTargetName, &cm); err != nil {
					return nil, err
				}

				return initConfigMapState(&cm, managedByMergeTargetAnnotation), nil
			}, timeout, interval).Should(Equal(expectedConfigMapState(mapRoles1, mapUsers1, "default/target")))
		})
	})

	Context("cleanup", func() {
		When("We remove the roles MergeSource", func() {
			Expect(rolesMergeSource).Should(Not(BeNil()))
			Expect(k8sClient.Delete(ctx, rolesMergeSource)).Should(Succeed())

			It("remotes the annotation and mapRoles data", func() {
				Eventually(func() (*configMapState, error) {
					cm := &corev1.ConfigMap{}
					err := k8sClient.Get(ctx, cmName, cm)
					if err != nil {
						return nil, errors.WithStack(err)
					}
					return initConfigMapState(cm, watchedByAnnotation), nil
				}, timeout, interval).
					Should(Equal(expectedConfigMapState("", mapUsers1, "default/aws-auth-map-users-source")))
			})
		})
	})

	It("cleans up annotations and data when we remove the source", func() {

		// delete the second source
		Expect(usersMergeSource).Should(Not(BeNil()))
		Expect(k8sClient.Delete(ctx, usersMergeSource)).Should(Succeed())

		Eventually(func() (bool, error) {
			cm := &corev1.ConfigMap{}
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test-cm1"}, cm)
			if err != nil {
				return false, errors.WithStack(err)
			}

			return cm.GetAnnotations()[watchedByAnnotation] == "", nil
		}, timeout, interval).Should(BeTrue())

		Eventually(func() bool {
			cm := &corev1.ConfigMap{}
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "aws-auth"}, cm)
			if err != nil {
				return false
			}

			Expect(cm.Data["mapUsers"]).Should(Equal(""))
			return true
		}).Should(BeTrue())
	})
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

const mapRoles1 = `
- rolearn: friend
  username: test1
  groups: [ group1, group2 ]
`
const mapUsers1 = `
- rolearn: friend
  username: test1
  groups: [ group1, group2 ]
`

type configMapState struct {
	MapRoles   string
	MapUsers   string
	Annotation string
}

func initConfigMapState(cm *corev1.ConfigMap, annotationName string) *configMapState {
	return &configMapState{
		MapRoles:   cm.Data["mapRoles"],
		MapUsers:   cm.Data["mapUsers"],
		Annotation: cm.GetAnnotations()[annotationName],
	}
}

func expectedConfigMapState(roles, users, annotation string) *configMapState {
	return &configMapState{
		MapRoles:   roles,
		MapUsers:   users,
		Annotation: annotation,
	}
}
