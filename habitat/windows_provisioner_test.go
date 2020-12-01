package habitat

import (
	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"testing"
)

func TestWindowsProvisioner_windowsInstallHabitat(t *testing.T) {
	cases := map[string]struct {
		Config   map[string]interface{}
		Commands map[string]bool
	}{
		"Install Habitat": {
			Config: map[string]interface{}{
				"license": "accept",
			},

			Commands: map[string]bool{
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept\\\"; [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12\"":                                                true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept\\\"; [System.Environment]::SetEnvironmentVariable(\\\"HAB_LICENSE\\\", \\\"accept\\\", [System.EnvironmentVariableTarget]::Machine)\"": true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept\\\"; [System.Environment]::SetEnvironmentVariable(\\\"HAB_LICENSE\\\", \\\"accept\\\", [System.EnvironmentVariableTarget]::Process)\"": true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept\\\"; [System.Environment]::SetEnvironmentVariable(\\\"HAB_LICENSE\\\", \\\"accept\\\", [System.EnvironmentVariableTarget]::User)\"":    true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept\\\"; irm https://raw.githubusercontent.com/habitat-sh/habitat/master/components/hab/install.ps1 > C:\\Windows\\TEMP\\install.ps1\"":    true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -File \"C:\\Windows\\TEMP\\install.ps1\" -Version latest":                                                                                                                                                                                       true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept\\\"; hab pkg install core/hab-sup\"":                                                                                             true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept\\\"; hab pkg install core/windows-service\"":                                                                                     true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept\\\"; New-NetFirewallRule -DisplayName \\\"Habitat TCP\\\" -Direction Inbound -Action Allow -Protocol TCP -LocalPort 9631,9638\"": true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept\\\"; New-NetFirewallRule -DisplayName \\\"Habitat UDP\\\" -Direction Inbound -Action Allow -Protocol UDP -LocalPort 9638\"":      true,
			},
		},
	}

	o := new(terraform.MockUIOutput)
	c := new(communicator.MockCommunicator)

	for k, tc := range cases {
		c.Commands = tc.Commands

		p, err := decodeConfig(
			schema.TestResourceDataRaw(t, Provision().(*schema.Provisioner).Schema, tc.Config),
		)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		err = p.windowsInstallHabitat(o, c)
		if err != nil {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}

func TestWindowsProvisioner_windowsStartHabitat(t *testing.T) {
	cases := map[string]struct {
		Config   map[string]interface{}
		Commands map[string]bool
	}{
		"Start Habitat": {
			Config: map[string]interface{}{
				"version":          "latest",
				"license":          "accept-no-persist",
				"auto_update":      false,
				"service_name":     "hab-sup",
				"peers":            []interface{}{"1.2.3.4", "5.6.7.8"},
				"ring_key":         "test-ring",
				"ring_key_content": "dead-beef",
				"event_stream": []interface{}{
					map[string]interface{}{
						"application":     "my-application",
						"environment":     "my-environment",
						"connect_timeout": "30",
						"meta": map[string]interface{}{
							"my-key1": "my-val1",
							"my-key2": "my-val-2",
						},
						"server_certificate": "dead-beef",
						"site":               "my-site",
						"token":              "ea7-beef",
						"url":                "https://automate.example.org",
					},
				},
			},

			Commands: map[string]bool{
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept-no-persist\\\"; $svcPath = Join-Path $env:SystemDrive \"hab\\svc\\windows-service\";[xml]$configXml = Get-Content (Join-Path $svcPath HabService.dll.config);$configXml.configuration.appSettings.ChildNodes[\"2\"].value = ' --peer 1.2.3.4 --peer 5.6.7.8 --ring test-ring --event-stream-application my-application --event-stream-environment my-environment --event-stream-connect-timeout 30 --event-meta \"my-key1=my-val1 my-key2=my-val-2\" --event-stream-server-certificate dead-beef --event-stream-site my-site --event-stream-token ea7-beef --event-stream-url https://automate.example.org --no-color';$configXml.Save((Join-Path $svcPath HabService.dll.config));\"": true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept-no-persist\\\"; Restart-Service Habitat\"": true,
			},
		},
	}

	o := new(terraform.MockUIOutput)
	c := new(communicator.MockCommunicator)

	for k, tc := range cases {
		c.Commands = tc.Commands

		p, err := decodeConfig(
			schema.TestResourceDataRaw(t, Provision().(*schema.Provisioner).Schema, tc.Config),
		)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		err = p.windowsStartHabitat(o, c)
		if err != nil {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}

func TestWindowsProvisioner_windowsUploadRingKey(t *testing.T) {
	cases := map[string]struct {
		Config   map[string]interface{}
		Commands map[string]bool
	}{
		"Upload Ring Key": {
			Config: map[string]interface{}{
				"license":          "accept",
				"service_name":     "hab-sup",
				"peers":            []interface{}{"1.2.3.4"},
				"ring_key":         "test-ring",
				"ring_key_content": "dead-beef",
			},

			Commands: map[string]bool{
				"powershell.exe -Command echo dead-beef | hab ring key import": true,
			},
		},
	}

	o := new(terraform.MockUIOutput)
	c := new(communicator.MockCommunicator)

	for k, tc := range cases {
		c.Commands = tc.Commands

		p, err := decodeConfig(
			schema.TestResourceDataRaw(t, Provision().(*schema.Provisioner).Schema, tc.Config),
		)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		err = p.windowsUploadRingKey(o, c)
		if err != nil {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}

func TestWindowsProvisioner_windowsUploadCtlSecret(t *testing.T) {
	cases := map[string]struct {
		Config   map[string]interface{}
		Commands map[string]bool
		Uploads  map[string]string
	}{
		"Upload Ctl Secret": {
			Config: map[string]interface{}{
				"license":      "accept",
				"service_name": "hab-sup",
				"peers":        []interface{}{"1.2.3.4"},
				"ctl_secret":   "dead-beef",
			},

			Commands: map[string]bool{
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept\\\"; mkdir C:\\hab\\sup\\default | out-null\"": true,
			},

			Uploads: map[string]string{
				"C:\\hab\\sup\\default\\CTL_SECRET": "dead-beef",
			},
		},
	}

	o := new(terraform.MockUIOutput)
	c := new(communicator.MockCommunicator)

	for k, tc := range cases {
		c.Commands = tc.Commands
		c.Uploads = tc.Uploads

		p, err := decodeConfig(
			schema.TestResourceDataRaw(t, Provision().(*schema.Provisioner).Schema, tc.Config),
		)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		err = p.windowsUploadCtlSecret(o, c)
		if err != nil {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}

func TestWindowsProvisioner_windowsStartHabitatService(t *testing.T) {
	cases := map[string]struct {
		Config   map[string]interface{}
		Commands map[string]bool
		Uploads  map[string]string
	}{
		"Start Habitat Services": {
			Config: map[string]interface{}{
				"version":          "latest",
				"license":          "accept-no-persist",
				"auto_update":      false,
				"service_name":     "hab-sup",
				"peers":            []interface{}{"1.2.3.4", "5.6.7.8"},
				"ring_key":         "test-ring",
				"ring_key_content": "dead-beef",
				"service": []interface{}{
					map[string]interface{}{
						"name":        "core/foo",
						"topology":    "standalone",
						"strategy":    "none",
						"channel":     "stable",
						"user_toml":   "[config]\nlisten = 0.0.0.0:8080",
						"service_key": "dead-beef\nabc1234567890",
						"bind": []interface{}{
							map[string]interface{}{
								"alias":   "backend",
								"service": "bar",
								"group":   "default",
							},
						},
					},
					map[string]interface{}{
						"name":        "core/bar",
						"topology":    "standalone",
						"strategy":    "rolling",
						"channel":     "staging",
						"user_toml":   "[config]\nlisten = 0.0.0.0:443",
						"service_key": "ea7-beef\ncba9876543210",
					},
				},
			},

			Commands: map[string]bool{
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept-no-persist\\\"; mkdir C:\\hab\\user\\foo\\config | out-null\"":                                                              true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept-no-persist\\\"; mkdir C:\\hab\\user\\bar\\config | out-null\"":                                                              true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept-no-persist\\\"; hab pkg install core/foo  --channel stable\"":                                                               true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept-no-persist\\\"; hab svc load core/foo  --topology standalone --strategy none --channel stable --bind backend:bar.default\"": true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept-no-persist\\\"; hab pkg install core/bar  --channel staging\"":                                                              true,
				"powershell.exe -NoProfile -ExecutionPolicy Bypass -Command \"$Env:HAB_NONINTERACTIVE=\\\"true\\\"; $Env:HAB_NOCOLORING=\\\"true\\\"; $Env:HAB_LICENSE=\\\"accept-no-persist\\\"; hab svc load core/bar  --topology standalone --strategy rolling --channel staging\"":                        true,
			},

			Uploads: map[string]string{
				"C:\\hab\\user\\bar\\config\\user.toml":       "[config]\nlisten = 0.0.0.0:443",
				"C:\\hab\\user\\foo\\config\\user.toml":       "[config]\nlisten = 0.0.0.0:8080",
				"C:\\hab\\cache\\keys\\abc1234567890.box.key": "dead-beef\nabc1234567890",
				"C:\\hab\\cache\\keys\\cba9876543210.box.key": "ea7-beef\ncba9876543210",
			},
		},
	}

	o := new(terraform.MockUIOutput)
	c := new(communicator.MockCommunicator)

	for k, tc := range cases {
		c.Commands = tc.Commands
		c.Uploads = tc.Uploads

		p, err := decodeConfig(
			schema.TestResourceDataRaw(t, Provision().(*schema.Provisioner).Schema, tc.Config),
		)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		var errs []error
		for _, s := range p.Services {
			err = p.windowsStartHabitatService(o, c, s)
			if err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			for _, e := range errs {
				t.Logf("Test %q failed: %v", k, e)
				t.Fail()
			}
		}
	}
}
