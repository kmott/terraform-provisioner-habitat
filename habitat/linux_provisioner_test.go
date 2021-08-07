package habitat

import (
	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"testing"
)

const linuxDefaultSystemdUnitFileContents = `[Unit]
Description=Habitat Supervisor

[Service]
ExecStart=/bin/hab sup run --peer 1.2.3.4 --auto-update --no-color
Restart=on-failure
[Install]
WantedBy=default.target`

const linuxCustomSystemdUnitFileContents = `[Unit]
Description=Habitat Supervisor

[Service]
ExecStart=/bin/hab sup run --listen-ctl 192.168.0.1:8443 --listen-gossip 192.168.10.1:9443 --listen-http 192.168.20.1:8080 --peer 1.2.3.4 --peer 5.6.7.8 --peer foo.example.com --event-stream-application my-application --event-stream-environment my-environment --event-stream-connect-timeout 30 --event-meta "my-key1=my-val1 my-key2=my-val-2" --event-stream-server-certificate dead-beef --event-stream-site my-site --event-stream-token ea7-beef --event-stream-url https://automate.example.org --no-color
Restart=on-failure
Environment="HAB_SUP_GATEWAY_AUTH_TOKEN=ea7-beef"
Environment="HAB_AUTH_TOKEN=dead-beef"
[Install]
WantedBy=default.target`

const linuxReStartHabitatSh = `#!/bin/bash
#
# This starts or re-starts Habitat to the running system.  Uploaded to /tmp/re-start-habitat.sh, and called by various 
# steps of the Linux Provisioner to configure the system.
#
__SERVICE_NAME="${1:-hab-supervisor.service}"
__UNIT_FILE="${2:-/etc/systemd/system/${__SERVICE_NAME}}"
__TMP_UNIT_FILE="${3:-/tmp/${__SERVICE_NAME}}"
__NEW_CHECKSUM="${4}"
__EXISTING_CHECKSUM=

if [[ -e "${__UNIT_FILE}" ]]; then
	__EXISTING_CHECKSUM="$( cat "${__UNIT_FILE}" | sha256sum - | awk -F ' ' '{print $1}' )"
fi

if [[ "${__EXISTING_CHECKSUM}" != "${__NEW_CHECKSUM}" ]]; then
	mv "${__TMP_UNIT_FILE}" "${__UNIT_FILE}"
	systemctl daemon-reload
	systemctl restart "${__SERVICE_NAME}"

	# Wait for hab-supervisor to come back up
	__RUNNING="$( hab svc status 2>/dev/null )"
	while [[ -z "${__RUNNING}" ]]; do
		echo "Waiting for Habitat to restart ..."
		sleep 5
		__RUNNING="$( hab svc status 2>/dev/null )"
	done
fi

systemctl enable "${__SERVICE_NAME}"`

func TestLinuxProvisioner_linuxInstallHabitat(t *testing.T) {
	cases := map[string]struct {
		Config   map[string]interface{}
		Commands map[string]bool
	}{
		"Installation with sudo": {
			Config: map[string]interface{}{
				"version":     "0.79.1",
				"auto_update": true,
				"use_sudo":    true,
			},

			Commands: map[string]bool{
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'curl --silent -L0 https://raw.githubusercontent.com/habitat-sh/habitat/master/components/hab/install.sh > install.sh'": true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'bash ./install.sh -v 0.79.1'":                                                                                          true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'hab pkg install core/busybox'":                                                                                         true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'hab pkg exec core/busybox adduser -D -g \"\" hab'":                                                                     true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'rm -f install.sh'":                                                                                                     true,
			},
		},
		"Installation without sudo": {
			Config: map[string]interface{}{
				"version":     "0.79.1",
				"auto_update": true,
				"use_sudo":    false,
			},

			Commands: map[string]bool{
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true /bin/bash -c 'curl --silent -L0 https://raw.githubusercontent.com/habitat-sh/habitat/master/components/hab/install.sh > install.sh'": true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true /bin/bash -c 'bash ./install.sh -v 0.79.1'":                                                                                          true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true /bin/bash -c 'hab pkg install core/busybox'":                                                                                         true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true /bin/bash -c 'hab pkg exec core/busybox adduser -D -g \"\" hab'":                                                                     true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true /bin/bash -c 'rm -f install.sh'":                                                                                                     true,
			},
		},
		"Installation with Habitat license acceptance": {
			Config: map[string]interface{}{
				"version":        "0.81.0",
				"accept_license": true,
				"auto_update":    true,
				"use_sudo":       true,
			},

			Commands: map[string]bool{
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'curl --silent -L0 https://raw.githubusercontent.com/habitat-sh/habitat/master/components/hab/install.sh > install.sh'": true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'bash ./install.sh -v 0.81.0'":                                                                                          true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'hab pkg install core/busybox'":                                                                                         true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'hab pkg exec core/busybox adduser -D -g \"\" hab'":                                                                     true,
				"env HAB_LICENSE=accept sudo -E /bin/bash -c 'hab -V'":                                                                                                                                        true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'rm -f install.sh'":                                                                                                     true,
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

		err = p.linuxInstallHabitat(o, c)
		if err != nil {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}

func TestLinuxProvisioner_linuxStartHabitat(t *testing.T) {
	cases := map[string]struct {
		Config   map[string]interface{}
		Commands map[string]bool
		Uploads  map[string]string
	}{
		"Start systemd Habitat with sudo": {
			Config: map[string]interface{}{
				"version":      "0.79.1",
				"auto_update":  true,
				"use_sudo":     true,
				"service_name": "hab-sup",
				"peers":        []interface{}{"1.2.3.4"},
			},

			Commands: map[string]bool{
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'hab pkg install core/hab-sup/0.79.1'":                                                                                                                                             true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'systemctl enable hab-sup'":                                                                                                                                                        true,
				`env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'bash /tmp/re-start-habitat.sh "hab-sup.service" "/etc/systemd/system/hab-sup.service" "/tmp/hab-sup.service" "6c974352a890774be845587207b417fd149fbc47c5bf85510ef338bd92002c49"'`: true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'rm /tmp/re-start-habitat.sh'":                                                                                                                                                     true,
			},

			Uploads: map[string]string{
				"/tmp/hab-sup.service":     linuxDefaultSystemdUnitFileContents,
				"/tmp/re-start-habitat.sh": linuxReStartHabitatSh,
			},
		},
		"Start systemd Habitat without sudo": {
			Config: map[string]interface{}{
				"version":      "0.79.1",
				"auto_update":  true,
				"use_sudo":     false,
				"service_name": "hab-sup",
				"peers":        []interface{}{"1.2.3.4"},
			},

			Commands: map[string]bool{
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true /bin/bash -c 'hab pkg install core/hab-sup/0.79.1'":                                                                                                                                             true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true /bin/bash -c 'systemctl enable hab-sup && systemctl start hab-sup'":                                                                                                                             true,
				`env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true /bin/bash -c 'bash /tmp/re-start-habitat.sh "hab-sup.service" "/etc/systemd/system/hab-sup.service" "/tmp/hab-sup.service" "6c974352a890774be845587207b417fd149fbc47c5bf85510ef338bd92002c49"'`: true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true /bin/bash -c 'systemctl enable hab-sup'":                                                                                                                                                        true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true /bin/bash -c 'rm /tmp/re-start-habitat.sh'":                                                                                                                                                     true,
			},

			Uploads: map[string]string{
				"/tmp/hab-sup.service":     linuxDefaultSystemdUnitFileContents,
				"/tmp/re-start-habitat.sh": linuxReStartHabitatSh,
			},
		},
		"Start unmanaged Habitat with sudo": {
			Config: map[string]interface{}{
				"version":      "0.81.0",
				"license":      "accept-no-persist",
				"auto_update":  true,
				"use_sudo":     true,
				"service_type": "unmanaged",
				"peers":        []interface{}{"1.2.3.4"},
			},

			Commands: map[string]bool{
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true HAB_LICENSE=accept-no-persist sudo -E /bin/bash -c 'hab pkg install core/hab-sup/0.81.0'":                                                                                                             true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true HAB_LICENSE=accept-no-persist sudo -E /bin/bash -c 'mkdir -p /hab/sup/default && chmod o+w /hab/sup/default'":                                                                                         true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true HAB_LICENSE=accept-no-persist sudo -E /bin/bash -c '(env HAB_LICENSE=accept-no-persist  setsid hab sup run --peer 1.2.3.4 --auto-update --no-color > /hab/sup/default/sup.log 2>&1 <&1 &) ; sleep 1'": true,
			},

			Uploads: map[string]string{
				"/etc/systemd/system/hab-sup.service": linuxDefaultSystemdUnitFileContents,
			},
		},
		"Start Habitat with custom config": {
			Config: map[string]interface{}{
				"version":            "0.79.1",
				"auto_update":        false,
				"use_sudo":           true,
				"service_name":       "hab-sup",
				"peer":               "--peer host1 --peer host2",
				"peers":              []interface{}{"1.2.3.4", "5.6.7.8", "foo.example.com"},
				"listen_ctl":         "192.168.0.1:8443",
				"listen_gossip":      "192.168.10.1:9443",
				"listen_http":        "192.168.20.1:8080",
				"builder_auth_token": "dead-beef",
				"gateway_auth_token": "ea7-beef",
				"ctl_secret":         "bad-beef",
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
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true HAB_AUTH_TOKEN=dead-beef sudo -E /bin/bash -c 'hab pkg install core/hab-sup/0.79.1'":                                                                                                                                             true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true HAB_AUTH_TOKEN=dead-beef sudo -E /bin/bash -c 'systemctl enable hab-sup'":                                                                                                                                                        true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true HAB_AUTH_TOKEN=dead-beef sudo -E /bin/bash -c 'mv /tmp/hab-sup.service /etc/systemd/system/hab-sup.service'":                                                                                                                     true,
				`env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true HAB_AUTH_TOKEN=dead-beef sudo -E /bin/bash -c 'bash /tmp/re-start-habitat.sh "hab-sup.service" "/etc/systemd/system/hab-sup.service" "/tmp/hab-sup.service" "a5a461dda1c265d6d279bc0c435eb5c51669afbf2986da6bd6062ddbe9664288"'`: true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true HAB_AUTH_TOKEN=dead-beef sudo -E /bin/bash -c 'rm /tmp/re-start-habitat.sh'":                                                                                                                                                     true,
			},

			Uploads: map[string]string{
				"/tmp/hab-sup.service":     linuxCustomSystemdUnitFileContents,
				"/tmp/re-start-habitat.sh": linuxReStartHabitatSh,
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

		err = p.linuxStartHabitat(o, c)
		if err != nil {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}

func TestLinuxProvisioner_linuxUploadRingKey(t *testing.T) {
	cases := map[string]struct {
		Config   map[string]interface{}
		Commands map[string]bool
	}{
		"Upload ring key": {
			Config: map[string]interface{}{
				"version":          "0.79.1",
				"auto_update":      true,
				"use_sudo":         true,
				"service_name":     "hab-sup",
				"peers":            []interface{}{"1.2.3.4"},
				"ring_key":         "test-ring",
				"ring_key_content": "dead-beef",
			},

			Commands: map[string]bool{
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'echo -e \"dead-beef\" | hab ring key import'": true,
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

		err = p.linuxUploadRingKey(o, c)
		if err != nil {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}

func TestLinuxProvisioner_linuxUploadCtlSecret(t *testing.T) {
	cases := map[string]struct {
		Config   map[string]interface{}
		Commands map[string]bool
		Uploads  map[string]string
	}{
		"Upload Ctl Secret": {
			Config: map[string]interface{}{
				"version":      "0.79.1",
				"auto_update":  true,
				"use_sudo":     true,
				"service_name": "hab-sup",
				"peers":        []interface{}{"1.2.3.4"},
				"ctl_secret":   "dead-beef",
			},

			Commands: map[string]bool{
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'mkdir -p /hab/sup/default'":                                                                                                               true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'mv /tmp/CTL_SECRET /hab/sup/default/CTL_SECRET && chown root:root /hab/sup/default/CTL_SECRET && chmod 0600 /hab/sup/default/CTL_SECRET'": true,
			},

			Uploads: map[string]string{
				"/tmp/CTL_SECRET":             "dead-beef",
				"/hab/sup/default/CTL_SECRET": "dead-beef",
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

		err = p.linuxUploadCtlSecret(o, c)
		if err != nil {
			t.Fatalf("Test %q failed: %v", k, err)
		}
	}
}

func TestLinuxProvisioner_linuxStartHabitatService(t *testing.T) {
	cases := map[string]struct {
		Config   map[string]interface{}
		Commands map[string]bool
		Uploads  map[string]string
	}{
		"Start Habitat service with sudo": {
			Config: map[string]interface{}{
				"version":          "0.79.1",
				"auto_update":      false,
				"use_sudo":         true,
				"service_name":     "hab-sup",
				"peers":            []interface{}{"1.2.3.4"},
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
						"reload":      "true",
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
						"reload":      "true",
					},
				},
			},

			Commands: map[string]bool{
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'hab pkg install core/foo  --channel stable'":                                                                        true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'mkdir -p /hab/user/foo/config'":                                                                                     true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'mv /tmp/user-a5b83ec1b302d109f41852ae17379f75c36dff9bc598aae76b6f7c9cd425fd76.toml /hab/user/foo/config/user.toml'": true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'hab svc status core/foo >/dev/null 2>&1'":                                                                           true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'hab svc unload core/bar ; sleep 3'":                                                                                 true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'hab svc load core/foo  --topology standalone --strategy none --channel stable --bind backend:bar.default'":          true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'hab pkg install core/bar  --channel staging'":                                                                       true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'mkdir -p /hab/user/bar/config'":                                                                                     true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'mv /tmp/user-6466ae3283ae1bd4737b00367bc676c6465b25682169ea5f7da222f3f078a5bf.toml /hab/user/bar/config/user.toml'": true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'hab svc unload core/foo ; sleep 3'":                                                                                 true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'hab svc load core/bar  --topology standalone --strategy rolling --channel staging'":                                 true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'mv /tmp/abc1234567890.box.key /hab/cache/keys/abc1234567890.box.key'":                                               true,
				"env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true sudo -E /bin/bash -c 'mv /tmp/cba9876543210.box.key /hab/cache/keys/cba9876543210.box.key'":                                               true,
			},

			Uploads: map[string]string{
				"/tmp/user-a5b83ec1b302d109f41852ae17379f75c36dff9bc598aae76b6f7c9cd425fd76.toml": "[config]\nlisten = 0.0.0.0:8080",
				"/tmp/user-6466ae3283ae1bd4737b00367bc676c6465b25682169ea5f7da222f3f078a5bf.toml": "[config]\nlisten = 0.0.0.0:443",
				"/tmp/abc1234567890.box.key": "dead-beef\nabc1234567890",
				"/tmp/cba9876543210.box.key": "ea7-beef\ncba9876543210",
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
			err = p.linuxStartHabitatService(o, c, s)
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
