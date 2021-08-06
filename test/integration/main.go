package test

import (
	"encoding/json"
	. "fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/gruntwork-io/terratest/modules/shell"
	"github.com/gruntwork-io/terratest/modules/ssh"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/masterzen/winrm"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	. "github.com/onsi/gomega/gstruct"
)

var (
	t                       = &testing.T{}
	terratestSkipCleanup, _ = strconv.ParseBool(os.Getenv("TERRATEST_SKIP_CLEANUP"))
	defaultTerraformOptions = terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: "../terraform",
	})
	terraformOutput *output

	// SSH
	sshPublicKeyFile  = Sprintf("%s/.ssh/klm-id_rsa.pub", os.Getenv("HOME"))
	sshPrivateKeyFile = Sprintf("%s/.ssh/klm-id_rsa.pem", os.Getenv("HOME"))
	sshKeyPair        *ssh.KeyPair
)

func init() {
	config.DefaultReporterConfig.SlowSpecThreshold = 10 * time.Minute.Seconds()
	format.MaxLength = 0
}

var _ = BeforeSuite(func() {
	// Setup SSH Keypair
	publicKeyFileContents, err := ioutil.ReadFile(sshPublicKeyFile)
	Expect(err).ShouldNot(HaveOccurred())
	privateKeyFileContents, err := ioutil.ReadFile(sshPrivateKeyFile)
	Expect(err).ShouldNot(HaveOccurred())
	sshKeyPair = &ssh.KeyPair{PublicKey: string(publicKeyFileContents), PrivateKey: string(privateKeyFileContents)}

	// Init and Apply Terraform
	_, err = terraform.InitAndApplyE(t, defaultTerraformOptions)
	Expect(err).ShouldNot(HaveOccurred())

	// Get output from Terraform
	output, err := terraform.OutputJsonE(t, defaultTerraformOptions, "")
	Expect(err).ShouldNot(HaveOccurred())

	// Parse Terraform output to struct
	terraformOutput, err = NewOutput(output)
	Expect(err).ShouldNot(HaveOccurred())

	//
	// Linux
	//
	var machine *machineOutputValue
	machine = terraformOutput.Linux.Get()
	Expect(*machine).Should(MatchFields(
		IgnoreExtras,
		Fields{
			"Address":       MatchRegexp(`^172\.16\.46(.*)$`),
			"CtlSecret":     Equal("dead-beef"),
			"GatewaySecret": Equal("ea7-beef"),
			"Id":            Not(BeEmpty()),
			"Name":          Equal("linux"),
			"SshUsername":   Not(BeEmpty()),
			"SshPassword":   Not(BeEmpty()),
			"WinrmUsername": BeEmpty(),
			"WinrmPassword": BeEmpty(),
			"InspecProfile": Equal("linux"),
		},
	))
	Expect(machine.Ready("stat /hab/svc/effortless/config/attributes.json && jq -V", 60, 5*time.Second)).ShouldNot(BeEmpty())

	//
	// Supervisor Ring
	//
	Expect(len(terraformOutput.SupervisorRing.Value)).Should(Equal(3))
	for k, _ := range terraformOutput.SupervisorRing.Value {
		machine = terraformOutput.SupervisorRing.GetValue(k)
		Expect(*machine).Should(MatchFields(
			IgnoreExtras,
			Fields{
				"Address":       MatchRegexp(`^172\.16\.46(.*)$`),
				"CtlSecret":     Equal("dead-beef"),
				"GatewaySecret": Equal("ea7-beef"),
				"Id":            Not(BeEmpty()),
				"Name":          Equal(Sprintf("sup-ring-%v", k+1)),
				"SshUsername":   Not(BeEmpty()),
				"SshPassword":   Not(BeEmpty()),
				"WinrmUsername": BeEmpty(),
				"WinrmPassword": BeEmpty(),
				"InspecProfile": Equal("linux"),
			},
		))
		Expect(machine.Ready("stat /hab/svc/effortless/config/attributes.json && jq -V", 60, 5*time.Second)).ShouldNot(BeEmpty())
	}

	//
	// Windows
	//
	machine = terraformOutput.Windows.Get()
	Expect(*machine).Should(MatchFields(
		IgnoreExtras,
		Fields{
			"Address":       MatchRegexp(`^172\.16\.46(.*)$`),
			"CtlSecret":     Equal("dead-beef"),
			"GatewaySecret": Equal("ea7-beef"),
			"Id":            Not(BeEmpty()),
			"Name":          Equal("windows"),
			"SshUsername":   BeEmpty(),
			"SshPassword":   BeEmpty(),
			"WinrmUsername": Not(BeEmpty()),
			"WinrmPassword": Not(BeEmpty()),
			"InspecProfile": Equal("windows"),
		},
	))
	// Because Powershell doesn't like me, we have 3 different commands we need to verify succeed before we can carry on
	Expect(machine.Ready("Get-Item C:/hab/svc/effortless/config/attributes.json", 60, 5*time.Second)).ShouldNot(BeEmpty())
	Expect(machine.Ready("hab pkg exec core/jq-static -- jq -V", 60, 5*time.Second)).ShouldNot(BeEmpty())
	Expect(machine.Ready("(Invoke-WebRequest -Headers @{'Authorization' = 'Bearer ea7-beef'} -Uri 'http://localhost:9631/census' -UseBasicParsing).Content", 60, 5*time.Second)).ShouldNot(BeEmpty())
})

var _ = AfterSuite(func() {
	if !terratestSkipCleanup {
		terraform.Destroy(t, defaultTerraformOptions)
	}
})

//
// TERRATEST HELPERS
//
func NewOutput(data string) (*output, error) {
	o := new(output)
	return o, json.Unmarshal([]byte(data), o)
}

type output struct {
	Linux          *machineOutput `json:"linux"`
	Windows        *machineOutput `json:"windows"`
	SupervisorRing *machineOutput `json:"supervisor-ring"`
}

type machineOutput struct {
	Value []*machineOutputValue `json:"value"`
}

func (mo *machineOutput) Get() *machineOutputValue {
	return mo.GetValue(0)
}

func (mo *machineOutput) GetValue(index int) *machineOutputValue {
	return mo.Value[index]
}

func (mo *machineOutput) GetMachinePeerString() string {
	var items = make([]string, 0)
	for _, v := range mo.Value {
		items = append(items, v.GetAddress())
	}

	return Sprintf("--peer %s", strings.Join(items, " --peer "))
}

type machineOutputValue struct {
	Address       string `json:"address"`
	CtlSecret     string `json:"ctl_secret"`
	GatewaySecret string `json:"gateway_secret"`
	Id            string `json:"id"`
	Name          string `json:"name"`
	SshUsername   string `json:"ssh_username,omitempty"`
	SshPassword   string `json:"ssh_password,omitempty"`
	WinrmUsername string `json:"winrm_username,omitempty"`
	WinrmPassword string `json:"winrm_password,omitempty"`
	InspecProfile string `json:"inspec_profile"`
}

func (mov *machineOutputValue) Ready(cmd string, maxRetries int, timeBetweenRetries time.Duration) string {
	if mov.InspecProfile == "windows" {
		return mov.windowsReady(cmd, maxRetries, timeBetweenRetries)
	}

	return mov.linuxReady(cmd, maxRetries, timeBetweenRetries)
}

func (mov *machineOutputValue) linuxReady(cmd string, maxRetries int, timeBetweenRetries time.Duration) string {
	host := ssh.Host{
		Hostname:    mov.GetAddress(),
		SshUserName: mov.GetUsername(),
		Password:    mov.GetPassword(),
		SshKeyPair:  sshKeyPair,
	}

	return strings.TrimSpace(retry.DoWithRetry(t, Sprintf("Waiting for command %q to succeed for determining machine ready", cmd), maxRetries, timeBetweenRetries, func() (string, error) {
		return ssh.CheckSshCommandE(t, host, cmd)
	}))
}

func (mov *machineOutputValue) windowsReady(cmd string, maxRetries int, timeBetweenRetries time.Duration) string {
	return strings.TrimSpace(retry.DoWithRetry(t, Sprintf("Waiting for command %q to succeed for determining machine ready", cmd), maxRetries, timeBetweenRetries, func() (string, error) {
		client, err := winrm.NewClient(winrm.NewEndpoint(mov.GetAddress(), 5986, true, true, nil, nil, nil, 0), mov.GetUsername(), mov.GetPassword())
		if err != nil {
			return "", err
		}

		// Didn't have time to try to figure it out, but sometimes powershell.exe commands return `nil` for `err`, even though
		// they failed.  So, we check to see if exitCode > 0 or err != nil or stdout == "" to determine if we need to re-try the cmd
		stdout, _, exitCode, err := client.RunWithString(Sprintf("powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"%s\"", cmd), "")
		if exitCode > 0 || err != nil || stdout == "" {
			return "", Errorf("Got error(%v) or non-zero exitCode(%v) or empty stdout(%v) for cmd, re-trying ...", err, exitCode, stdout)
		}

		return stdout, err
	}))
}

func (mov *machineOutputValue) Inspec() (string, error) {
	var args []string

	switch mov.InspecProfile {
	case "windows":
		args = []string{"exec", "inspec", "exec", mov.GetInspecProfile(), "-t", mov.GetConnectionString(), "--password", mov.GetPassword(), "--ssl", "--self-signed", "--no-color"}
	default:
		args = []string{"exec", "inspec", "exec", mov.GetInspecProfile(), "-t", mov.GetConnectionString(), "--password", mov.GetPassword(), "-i", sshPrivateKeyFile, "--no-color"}
	}

	return shell.RunCommandAndGetOutputE(t, shell.Command{
		Command:    "chef",
		Args:       args,
		WorkingDir: "../inspec",
	})
}

func (mov *machineOutputValue) GetConnectionString() string {
	switch mov.InspecProfile {
	case "windows":
		return Sprintf("winrm://%v@%v", mov.GetUsername(), mov.GetAddress())
	}

	return Sprintf("ssh://%v@%v", mov.GetUsername(), mov.GetAddress())
}

func (mov *machineOutputValue) GetAddress() string {
	return mov.Address
}

func (mov *machineOutputValue) GetName() string {
	return mov.Name
}

func (mov *machineOutputValue) GetInspecProfile() string {
	return mov.InspecProfile
}

func (mov *machineOutputValue) GetUsername() string {
	if mov.SshUsername != "" {
		return mov.SshUsername
	}

	if mov.WinrmUsername != "" {
		return mov.WinrmUsername
	}

	return ""
}

func (mov *machineOutputValue) GetPassword() string {
	if mov.SshPassword != "" {
		return mov.SshPassword
	}

	if mov.WinrmPassword != "" {
		return mov.WinrmPassword
	}

	return ""
}
