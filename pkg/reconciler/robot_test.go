/*
Copyright 2019 The Authors.

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

package reconciler

import (
	"time"

	crdv1 "github.com/moolen/harbor-sync/api/v1"
	"github.com/moolen/harbor-sync/pkg/harbor"
	harborfake "github.com/moolen/harbor-sync/pkg/harbor/fake"
	"github.com/moolen/harbor-sync/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe("Controller", func() {

	var harborClient harborfake.Client
	mapping := crdv1.ProjectMapping{
		Namespace: "foo",
		Secret:    "foo",
		Type:      crdv1.TranslateMappingType,
	}
	harborProject := harbor.Project{
		ID:   1,
		Name: "foo",
	}
	createdAccount := &harbor.CreateRobotResponse{
		Name:  "robot$sync-bot",
		Token: "1234",
	}
	log := zap.Logger(false)

	BeforeEach(func() {
		harborClient = harborfake.Client{
			CreateRobotAccountFunc: func(name string, project harbor.Project) (*harbor.CreateRobotResponse, error) {
				return createdAccount, nil
			},
			GetRobotAccountsFunc: func(project harbor.Project) ([]harbor.Robot, error) {
				return []harbor.Robot{
					{
						Name:      "robot$sync-bot",
						ExpiresAt: int(time.Now().Unix()),
					},
				}, nil
			},
		}
	})

	Describe("Robot", func() {

		It("should reconcile robot accounts", func(done Done) {
			cfg := test.EnsureHarborSyncConfigWithParams(k8sClient, "my-cfg", "my-project", &mapping, nil)
			harborClient.GetRobotAccountsFunc = nil
			credentials, changed, err := ReconcileRobotAccounts(harborClient, log, &cfg, harborProject, cfg.Spec.RobotAccountSuffix)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(*credentials).To(Equal(crdv1.RobotAccountCredential{
				Name:  createdAccount.Name,
				Token: createdAccount.Token,
			}))
			Expect(cfg.Status.RobotCredentials["foo"]).To(Equal(crdv1.RobotAccountCredential{
				Name:  createdAccount.Name,
				Token: createdAccount.Token,
			}))
			close(done)
		})

		It("should use robot credentials from status field", func(done Done) {
			cfg := test.EnsureHarborSyncConfigWithParams(k8sClient, "my-cfg", "my-project", &mapping, nil)
			harborClient.CreateRobotAccountFunc = nil
			cfg.Status.RobotCredentials = make(map[string]crdv1.RobotAccountCredential)
			cfg.Status.RobotCredentials["foo"] = crdv1.RobotAccountCredential{Name: "robot$sync-bot", Token: "bar"}
			credentials, changed, err := ReconcileRobotAccounts(harborClient, log, &cfg, harborProject, cfg.Spec.RobotAccountSuffix)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeFalse())
			Expect(*credentials).To(Equal(crdv1.RobotAccountCredential{
				Name:  "robot$sync-bot",
				Token: "bar",
			}))
			Expect(cfg.Status.RobotCredentials["foo"]).To(Equal(crdv1.RobotAccountCredential{
				Name:  "robot$sync-bot",
				Token: "bar",
			}))
			close(done)
		})

		It("should delete robot account when credentials are missing", func(done Done) {
			cfg := test.EnsureHarborSyncConfigWithParams(k8sClient, "my-cfg", "my-project", &mapping, nil)
			var deleteCalled bool
			harborClient.DeleteRobotAccountFunc = func(project harbor.Project, robotID int) error {
				deleteCalled = true
				return nil
			}
			cfg.Status.RobotCredentials = nil
			credentials, changed, err := ReconcileRobotAccounts(harborClient, log, &cfg, harborProject, cfg.Spec.RobotAccountSuffix)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(deleteCalled).To(BeTrue())
			Expect(*credentials).To(Equal(crdv1.RobotAccountCredential{
				Name:  createdAccount.Name,
				Token: createdAccount.Token,
			}))
			Expect(cfg.Status.RobotCredentials["foo"]).To(Equal(crdv1.RobotAccountCredential{
				Name:  createdAccount.Name,
				Token: createdAccount.Token,
			}))
			close(done)
		})

		It("should re-create disabled account", func(done Done) {
			cfg := test.EnsureHarborSyncConfigWithParams(k8sClient, "my-cfg", "my-project", &mapping, nil)
			harborClient.GetRobotAccountsFunc = func(project harbor.Project) ([]harbor.Robot, error) {
				return []harbor.Robot{
					{
						Name:      "robot$sync-bot",
						Disabled:  true,
						ExpiresAt: int(time.Now().Add(time.Hour * 24).Unix()),
					},
				}, nil
			}
			var deleteCalled bool
			harborClient.DeleteRobotAccountFunc = func(project harbor.Project, robotID int) error {
				deleteCalled = true
				return nil
			}
			cfg.Status.RobotCredentials = nil
			credentials, changed, err := ReconcileRobotAccounts(harborClient, log, &cfg, harborProject, cfg.Spec.RobotAccountSuffix)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(deleteCalled).To(BeTrue())
			Expect(*credentials).To(Equal(crdv1.RobotAccountCredential{
				Name:  createdAccount.Name,
				Token: createdAccount.Token,
			}))
			Expect(cfg.Status.RobotCredentials["foo"]).To(Equal(crdv1.RobotAccountCredential{
				Name:  createdAccount.Name,
				Token: createdAccount.Token,
			}))
			close(done)
		})

		It("should re-create expiring account", func(done Done) {
			cfg := test.EnsureHarborSyncConfigWithParams(k8sClient, "my-cfg", "my-project", &mapping, nil)
			harborClient.GetRobotAccountsFunc = func(project harbor.Project) ([]harbor.Robot, error) {
				return []harbor.Robot{
					{
						Name:      "robot$sync-bot",
						ExpiresAt: 0,
					},
				}, nil
			}
			var deleteCalled bool
			harborClient.DeleteRobotAccountFunc = func(project harbor.Project, robotID int) error {
				deleteCalled = true
				return nil
			}
			cfg.Status.RobotCredentials = nil
			credentials, changed, err := ReconcileRobotAccounts(harborClient, log, &cfg, harborProject, cfg.Spec.RobotAccountSuffix)
			Expect(err).ToNot(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(deleteCalled).To(BeTrue())
			Expect(*credentials).To(Equal(crdv1.RobotAccountCredential{
				Name:  createdAccount.Name,
				Token: createdAccount.Token,
			}))
			Expect(cfg.Status.RobotCredentials["foo"]).To(Equal(crdv1.RobotAccountCredential{
				Name:  createdAccount.Name,
				Token: createdAccount.Token,
			}))
			close(done)
		})
	})
})