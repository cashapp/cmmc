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

var _ = Describe("MergeSource/Target controllers", func() {
	const (
		timeout = time.Second * 15
		// duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	const mapRoles1 = `
- rolearn: friend
  usernme: test1
  groups: [ group1, group2 ]
`

	var mergeSource *cmmcv1beta1.MergeSource

	Context("When setting up the MergeSource and ConfigMaps", func() {
		ctx := context.Background()
		It("should be fine to set up a ConfigMap", func() {
			cm := &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind: "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm1",
					Namespace: "default",
					Labels: map[string]string{
						"test-label": "map-roles",
					},
				},
				Data: map[string]string{
					"mapRoles": mapRoles1,
				},
			}
			Expect(k8sClient.Create(ctx, cm)).Should(Succeed())
		})

		It("Annotates the created ConfigMap", func() {
			By("creating a MergeSource")
			mergeSource = &cmmcv1beta1.MergeSource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "config.cmmc.k8s.cash.app/v1beta1",
					Kind:       "MergeSource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aws-auth-map-roles-source",
					Namespace: "default",
				},
				Spec: cmmcv1beta1.MergeSourceSpec{
					Selector: map[string]string{
						"test-label": "map-roles",
					},
					Source: cmmcv1beta1.MergeSourceSourceSpec{
						Data: "mapRoles",
					},
					Target: cmmcv1beta1.MergeSourceTargetSpec{
						Name: "aws-auth-target",
						Data: "mapRoles",
					},
				},
			}
			Expect(k8sClient.Create(ctx, mergeSource)).Should(Succeed())

			Eventually(func() string {
				cm := &corev1.ConfigMap{}
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "test-cm1"}, cm)
				if err != nil {
					return ""
				}
				return cm.GetAnnotations()[watchedByAnnotation]
			}, timeout, interval).Should(Equal("default/aws-auth-map-roles-source"))
		})

		It("creates a target cm", func() {
			By("creating a MergeTarget")
			target := &cmmcv1beta1.MergeTarget{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "config.cmmc.k8s.cash.app/v1beta1",
					Kind:       "MergeTarget",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aws-auth-target",
					Namespace: "default",
				},
				Spec: cmmcv1beta1.MergeTargetSpec{
					Target: "default/aws-auth",
					Data: map[string]cmmcv1beta1.MergeTargetDataSpec{
						"mapRoles": {Init: ""},
					},
				},
			}

			Expect(k8sClient.Create(ctx, target)).Should(Succeed())

			Eventually(func() bool {
				cm := &corev1.ConfigMap{}
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: "aws-auth"}, cm)
				if err != nil {
					return false
				}

				Expect(cm.GetAnnotations()[managedByMergeTargetAnnotation]).Should(Equal("default/aws-auth-target"))
				Expect(cm.Data["mapRoles"]).Should(Equal(mapRoles1))
				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("cleans up annotations and data when we remove the source", func() {
			defer GinkgoRecover()
			Expect(mergeSource).Should(Not(BeNil()))
			Expect(k8sClient.Delete(ctx, mergeSource)).Should(Succeed())
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

				Expect(cm.Data["mapRoles"]).Should(Equal(""))
				return true
			}).Should(BeTrue())
		})
	})
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
