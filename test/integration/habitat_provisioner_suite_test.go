package test

import (
	"github.com/gruntwork-io/terratest/modules/terraform"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
	"time"
)

func TestHabitatProvisioner(test *testing.T) {
	RegisterFailHandlerWithT(test, Fail)
	RunSpecs(test, "Habitat Provisioner Suite")
	t = test
}

var _ = Describe("Habitat Provisioner", func() {
	defer GinkgoRecover()

	Context("Linux", func() {
		It("Successfully runs Inspec profile(s)", func() {
			output, err := terraformOutput.Linux.Get().Inspec()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(output).Should(MatchRegexp(`0 failures`))
		})
	})

	Context("Supervisor Ring", func() {
		It("Successfully runs Inspec profile(s)", func() {
			for _, machine := range terraformOutput.SupervisorRing.Value {
				output, err := machine.Inspec()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(output).Should(MatchRegexp(`0 failures`))
			}
		})
	})

	Context("Windows", func() {
		It("Successfully runs Inspec profile(s)", func() {
			output, err := terraformOutput.Windows.Get().Inspec()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(output).Should(MatchRegexp(`0 failures`))
		})
	})

	Context("Reprovision Supervisor Ring Node", func() {
		It("Successfully taints vsphere_virtual_machine.supervisor-ring[1]", func() {
			_, err := terraform.RunTerraformCommandE(t, defaultTerraformOptions, "taint", "vsphere_virtual_machine.supervisor-ring[1]")
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Successfully re-runs apply", func() {
			// Re-apply terraform state change
			output, err := terraform.InitAndApplyE(t, defaultTerraformOptions)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(output).ShouldNot(ContainSubstring("null_resource.habitat-provisioner[0] (habitat): Unloading service klm/effortless due to reload"))
			Expect(output).ShouldNot(ContainSubstring("null_resource.habitat-provisioner[1] (habitat): Unloading service klm/effortless due to reload"))
			Expect(output).Should(ContainSubstring("null_resource.habitat-provisioner[2] (habitat): Unloading service klm/effortless due to reload"))

			// Get output from Terraform
			output, err = terraform.OutputJsonE(t, defaultTerraformOptions, "")
			Expect(err).ShouldNot(HaveOccurred())

			// Parse Terraform output to struct
			terraformOutput, err = NewOutput(output)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Successfully re-runs Inspec profile(s) - Linux", func() {
			output, err := terraformOutput.Linux.Get().Inspec()
			Expect(terraformOutput.Linux.Get().Ready("stat /hab/svc/effortless/config/attributes.json && jq -V", 60, 5*time.Second)).ShouldNot(BeEmpty())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(output).Should(MatchRegexp(`0 failures`))
		})

		It("Successfully re-runs Inspec profile(s) - Supervisor Ring", func() {
			for _, machine := range terraformOutput.SupervisorRing.Value {
				Expect(machine.Ready("stat /hab/svc/effortless/config/attributes.json && jq -V", 60, 5*time.Second)).ShouldNot(BeEmpty())
				output, err := machine.Inspec()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(output).Should(MatchRegexp(`0 failures`))
			}
		})
	})
})
