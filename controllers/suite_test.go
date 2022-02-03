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
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gtypes "github.com/onsi/gomega/types"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	"github.com/cashapp/cmmc/util"
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
		mergeTarget      *cmmcv1beta1.MergeTarget

		ctx   = context.Background()
		names = struct {
			sourceCM1,
			targetCM,
			rolesSource,
			usersSource,
			target types.NamespacedName
		}{
			sourceCM1:   util.MustNamespacedName("default/test-cm-1", ""),
			targetCM:    util.MustNamespacedName("default/merge-me", ""),
			rolesSource: util.MustNamespacedName("default/map-roles-source", ""),
			usersSource: util.MustNamespacedName("default/map-users-source", ""),
			target:      util.MustNamespacedName("default/target", ""),
		}

		// selector is the standard label/selector we are going to be
		// using for our tests to watch ConfigMaps
		selector = map[string]string{
			"test-label": "for-this-source",
		}
	)

	Context("running the operator", func() {

		assertConfigMapState := func(name types.NamespacedName, ms ...interface{}) {
			Eventually(
				func() (*configMapState, error) {
					var cm corev1.ConfigMap
					if err := k8sClient.Get(ctx, name, &cm); err != nil {
						return nil, err
					}
					anns := cm.GetAnnotations()
					watchedBy := strings.Split(anns[watchedByAnnotation], ",")
					sort.Strings(watchedBy)
					return &configMapState{
						MapRoles:            cm.Data["mapRoles"],
						MapUsers:            cm.Data["mapUsers"],
						WatchedByAnnotation: watchedBy,
						ManagedByAnnotation: anns[managedByMergeTargetAnnotation],
					}, nil
				},
				timeout,
				interval,
			).Should(HaveConfigMapState(ms...))
		}

		It("should first have a source ConfigMap", func() {
			Expect(k8sClient.Create(ctx, &corev1.ConfigMap{
				TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap"},
				ObjectMeta: metaFromName(names.sourceCM1, selector),
				Data:       map[string]string{"mapRoles": mapRoles1, "mapUsers": mapUsers1},
			})).Should(Succeed())
		})

		When("creating MergeSource", func() {
			It("can create a MergeSource for users", func() {
				rolesMergeSource = cmmcv1beta1.NewMergeSource(names.rolesSource, cmmcv1beta1.MergeSourceSpec{
					Selector: selector,
					Source:   cmmcv1beta1.MergeSourceSourceSpec{Data: "mapRoles"},
					Target:   cmmcv1beta1.MergeSourceTargetSpec{Name: names.target.String(), Data: "mapRoles"},
				})
				Expect(k8sClient.Create(ctx, rolesMergeSource)).Should(Succeed())
			})

			It("can create a MergeSource for roles", func() {
				usersMergeSource = cmmcv1beta1.NewMergeSource(names.usersSource, cmmcv1beta1.MergeSourceSpec{
					Selector: selector,
					Source:   cmmcv1beta1.MergeSourceSourceSpec{Data: "mapUsers"},
					Target:   cmmcv1beta1.MergeSourceTargetSpec{Name: names.target.String(), Data: "mapUsers"},
				})
				Expect(k8sClient.Create(ctx, usersMergeSource)).Should(Succeed())
			})

			It("should annotate the source ConfigMap", func() {
				assertConfigMapState(
					names.sourceCM1,
					WatchedByAnnotation("default/map-roles-source,default/map-users-source"),
				)
			})
		})

		When("creating a MergeTarget", func() {
			It("can successfuly create a MergeTarget", func() {
				mergeTarget = cmmcv1beta1.NewMergeTarget(names.target, cmmcv1beta1.MergeTargetSpec{
					Target: names.targetCM.String(),
					Data:   map[string]cmmcv1beta1.MergeTargetDataSpec{"mapRoles": {}, "mapUsers": {}},
				})
				Expect(k8sClient.Create(ctx, mergeTarget)).Should(Succeed())
			})

			It("should create a target ConfigMap based on the MergeTarget", func() {
				assertConfigMapState(
					names.targetCM,
					MapRoles(mapRoles1),
					MapUsers(mapUsers1),
					ManagedByAnnotation(names.target.String()),
				)
			})
		})

		Context("cleanup", func() {
			When("removing roles MergeSource", func() {
				It("can be deleted", func() {
					Expect(rolesMergeSource).Should(Not(BeNil()))
					Expect(k8sClient.Delete(ctx, rolesMergeSource)).Should(Succeed())
				})

				It("removes the annotation for roles source", func() {
					assertConfigMapState(names.sourceCM1, WatchedByAnnotation(names.usersSource.String()))
				})

				It("removes the roles from the target", func() {
					assertConfigMapState(names.targetCM,
						ManagedByAnnotation(names.target.String()),
						MapRoles(""),
						MapUsers(mapUsers1),
					)
				})
			})

			When("removing users MergeSource", func() {
				It("can be deleted", func() {
					Expect(usersMergeSource).Should(Not(BeNil()))
					Expect(k8sClient.Delete(ctx, usersMergeSource)).Should(Succeed())
				})

				It("removes the annotation for users source", func() {
					assertConfigMapState(names.sourceCM1, WatchedByAnnotation(""))
				})

				It("removes the users from the target", func() {
					assertConfigMapState(names.targetCM,
						ManagedByAnnotation(names.target.String()),
						MapUsers(""),
						MapRoles(""),
					)
				})
			})

			When("removing the MergeTarget", func() {
				It("it can be deleted", func() {
					Expect(mergeTarget).Should(Not(BeNil()))
					Expect(k8sClient.Delete(ctx, mergeTarget)).Should(Succeed())
				})

				It("removes the target", func() {
					Eventually(
						func() (bool, error) {
							var cm corev1.ConfigMap
							err := k8sClient.Get(ctx, names.targetCM, &cm)
							if k8serrors.IsNotFound(err) {
								return true, nil
							}
							return false, err
						},
						timeout,
						interval,
					).Should(BeTrue())
				})
			})
		})
	})
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

type configMapState struct {
	MapRoles            string
	MapUsers            string
	ManagedByAnnotation string
	WatchedByAnnotation []string
}

type ManagedByAnnotation string
type WatchedByAnnotation string
type MapRoles string
type MapUsers string

func HaveConfigMapState(params ...interface{}) gtypes.GomegaMatcher {
	matchers := []gtypes.GomegaMatcher{}
	for _, p := range params {
		switch v := p.(type) {
		case ManagedByAnnotation:
			matchers = append(matchers, HaveField("ManagedByAnnotation", Equal(string(v))))
		case WatchedByAnnotation:
			// the order of the watched by annotation is suspect!
			// we want to have the _set_ of the annotation be valid
			// not necessarily the order.
			ss := strings.Split(string(v), ",")
			sort.Strings(ss)
			matchers = append(matchers, HaveField("WatchedByAnnotation", Equal(ss)))
		case MapRoles:
			matchers = append(matchers, HaveField("MapRoles", Equal(string(v))))
		case MapUsers:
			matchers = append(matchers, HaveField("MapUsers", Equal(string(v))))
		default:
			Fail(fmt.Sprintf("Unknown type %T in HaveConfigMapState() \n", v))
		}
	}

	return And(matchers...)
}

func metaFromName(n types.NamespacedName, labels map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Namespace: n.Namespace, Name: n.Name, Labels: labels}
}

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
