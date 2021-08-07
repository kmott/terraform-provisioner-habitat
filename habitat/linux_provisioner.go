package habitat

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/terraform"
)

const installURL = "https://raw.githubusercontent.com/habitat-sh/habitat/master/components/hab/install.sh"
const systemdUnit = `[Unit]
Description=Habitat Supervisor

[Service]
ExecStart=/bin/hab sup run{{ .SupOptions }}
Restart=on-failure
{{ if .GatewayAuthToken -}}
Environment="HAB_SUP_GATEWAY_AUTH_TOKEN={{ .GatewayAuthToken }}"
{{ end -}}
{{ if .BuilderAuthToken -}}
Environment="HAB_AUTH_TOKEN={{ .BuilderAuthToken }}"
{{ end -}}
{{ if .License -}}
Environment="HAB_LICENSE={{ .License }}"
{{ end -}}

[Install]
WantedBy=default.target
`

const startHabitatScript = `#!/bin/bash
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

systemctl enable "${__SERVICE_NAME}"
`

func (p *provisioner) linuxInstallHabitat(o terraform.UIOutput, comm communicator.Communicator) error {
	// Download the hab installer
	if err := p.runCommand(o, comm, p.linuxGetCommand(fmt.Sprintf("curl --silent -L0 %s > install.sh", installURL))); err != nil {
		return err
	}

	// Run the install script
	var command string
	if p.Version == "" {
		command = "bash ./install.sh "
	} else {
		command = fmt.Sprintf("bash ./install.sh -v %s", p.Version)
	}

	if err := p.runCommand(o, comm, p.linuxGetCommand(command)); err != nil {
		return err
	}

	// Create the hab user
	if err := p.createHabUser(o, comm); err != nil {
		return err
	}

	// Cleanup the installer
	return p.runCommand(o, comm, p.linuxGetCommand("rm -f install.sh"))
}

func (p *provisioner) createHabUser(o terraform.UIOutput, comm communicator.Communicator) error {
	var addUser bool

	// Install busybox to get us the user tools we need
	if err := p.runCommand(o, comm, p.linuxGetCommand("hab pkg install core/busybox")); err != nil {
		return err
	}

	// Check for existing hab user
	if err := p.runCommand(o, comm, p.linuxGetCommand("hab pkg exec core/busybox id hab")); err != nil {
		o.Output("No existing hab user detected, creating...")
		addUser = true
	}

	if addUser {
		return p.runCommand(o, comm, p.linuxGetCommand("hab pkg exec core/busybox adduser -D -g \"\" hab"))
	}

	return nil
}

func (p *provisioner) linuxStartHabitat(o terraform.UIOutput, comm communicator.Communicator) error {
	// Install the supervisor first
	var command string
	if p.Version == "latest" {
		command += p.linuxGetCommand("hab pkg install core/hab-sup")
	} else {
		command += p.linuxGetCommand(fmt.Sprintf("hab pkg install core/hab-sup/%s", p.Version))
	}

	if err := p.runCommand(o, comm, command); err != nil {
		return err
	}

	// Build up supervisor options
	options := ""
	if p.PermanentPeer {
		options += " --permanent-peer"
	}

	if p.ListenCtl != "" {
		options += fmt.Sprintf(" --listen-ctl %s", p.ListenCtl)
	}

	if p.ListenGossip != "" {
		options += fmt.Sprintf(" --listen-gossip %s", p.ListenGossip)
	}

	if p.ListenHTTP != "" {
		options += fmt.Sprintf(" --listen-http %s", p.ListenHTTP)
	}

	if len(p.Peers) > 0 {
		if len(p.Peers) == 1 {
			options += fmt.Sprintf(" --peer %s", p.Peers[0])
		} else {
			options += fmt.Sprintf(" --peer %s", strings.Join(p.Peers, " --peer "))
		}
	}

	if p.RingKey != "" {
		options += fmt.Sprintf(" --ring %s", p.RingKey)
	}

	if p.URL != "" {
		options += fmt.Sprintf(" --url %s", p.URL)
	}

	if p.Channel != "" {
		options += fmt.Sprintf(" --channel %s", p.Channel)
	}

	if p.Events != "" {
		options += fmt.Sprintf(" --events %s", p.Events)
	}

	if p.Organization != "" {
		options += fmt.Sprintf(" --org %s", p.Organization)
	}

	if p.HttpDisable {
		options += " --http-disable"
	}

	if p.AutoUpdate {
		options += " --auto-update"
	}

	if p.EventStream != nil {
		options += p.EventStream.FlagValues()
	}

	options += " --no-color"

	p.SupOptions = options

	// Start hab depending on service type
	switch p.ServiceType {
	case "unmanaged":
		return p.linuxStartHabitatUnmanaged(o, comm, options)
	case "systemd":
		return p.linuxStartHabitatSystemd(o, comm, options)
	default:
		return errors.New("unsupported service type")
	}
}

// This func is a little different than the others since we need to expose HAB_AUTH_TOKEN to a shell
// sub-process that's actually running the supervisor.
func (p *provisioner) linuxStartHabitatUnmanaged(o terraform.UIOutput, comm communicator.Communicator, options string) error {
	var token string
	var license string

	// Create the sup directory for the log file
	if err := p.runCommand(o, comm, p.linuxGetCommand("mkdir -p /hab/sup/default && chmod o+w /hab/sup/default")); err != nil {
		return err
	}

	// Set HAB_AUTH_TOKEN if provided
	if p.BuilderAuthToken != "" {
		token = fmt.Sprintf("env HAB_AUTH_TOKEN=%s ", p.BuilderAuthToken)
	}

	// Set HAB_LICENSE if provided
	if p.License != "" {
		license = fmt.Sprintf("HAB_LICENSE=%s ", p.License)
	}

	return p.runCommand(o, comm, p.linuxGetCommand(fmt.Sprintf("(env %s%s setsid hab sup run%s > /hab/sup/default/sup.log 2>&1 <&1 &) ; sleep 1", token, license, options)))
}

func (p *provisioner) linuxStartHabitatSystemd(o terraform.UIOutput, comm communicator.Communicator, options string) error {
	// Upload script
	if err := comm.Upload("/tmp/re-start-habitat.sh", strings.NewReader(startHabitatScript)); err != nil {
		return err
	}

	// Create a new template and parse the client config into it
	unitString := template.Must(template.New(fmt.Sprintf("%s.service", p.ServiceName)).Parse(systemdUnit))
	tempDestination := fmt.Sprintf("/tmp/%s.service", p.ServiceName)
	destination := fmt.Sprintf("/etc/systemd/system/%s.service", p.ServiceName)

	// Checksum for unit string
	var checksumBuf, fileBuf bytes.Buffer
	writer := io.MultiWriter(&checksumBuf, &fileBuf)
	err := unitString.Execute(writer, p)
	if err != nil {
		return fmt.Errorf("error executing %s.service template: %s", p.ServiceName, err)
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, bytes.NewReader(checksumBuf.Bytes())); err != nil {
		return err
	}
	newChecksum := hash.Sum(nil)

	if err := p.linuxUploadSystemdUnit(o, comm, tempDestination, &fileBuf); err != nil {
		return err
	}

	// Check for (re)start
	if err := p.runCommand(o, comm, p.linuxGetCommand(fmt.Sprintf("bash /tmp/re-start-habitat.sh \"%s.service\" \"%s\" \"%s\" \"%x\"", p.ServiceName, destination, tempDestination, newChecksum))); err != nil {
		return err
	}

	// Enable the service
	if err := p.runCommand(o, comm, p.linuxGetCommand(fmt.Sprintf("systemctl enable %s", p.ServiceName))); err != nil {
		return err
	}

	return p.runCommand(o, comm, p.linuxGetCommand("rm /tmp/re-start-habitat.sh"))
}

func (p *provisioner) linuxUploadSystemdUnit(o terraform.UIOutput, comm communicator.Communicator, tempDestination string, contents *bytes.Buffer) error {
	return comm.Upload(tempDestination, contents)
}

func (p *provisioner) linuxUploadRingKey(o terraform.UIOutput, comm communicator.Communicator) error {
	return p.runCommand(o, comm, p.linuxGetCommand(fmt.Sprintf(`echo -e "%s" | hab ring key import`, p.RingKeyContent)))
}

func (p *provisioner) linuxUploadCtlSecret(o terraform.UIOutput, comm communicator.Communicator) error {
	destination := "/hab/sup/default/CTL_SECRET"
	// Create the destination directory
	err := p.runCommand(o, comm, p.linuxGetCommand(fmt.Sprintf("mkdir -p %s", filepath.Dir(destination))))
	if err != nil {
		return err
	}

	keyContent := strings.NewReader(p.CtlSecret)
	if p.UseSudo {
		tempPath := "/tmp/CTL_SECRET"
		if err := comm.Upload(tempPath, keyContent); err != nil {
			return err
		}

		return p.runCommand(o, comm, p.linuxGetCommand(fmt.Sprintf("mv %s %s && chown root:root %s && chmod 0600 %s", tempPath, destination, destination, destination)))
	}

	return comm.Upload(destination, keyContent)
}

func (p *provisioner) linuxStartHabitatService(o terraform.UIOutput, comm communicator.Communicator, service Service) error {
	var options string

	if err := p.linuxInstallHabitatPackage(o, comm, service); err != nil {
		return err
	}

	if strings.TrimSpace(service.UserTOML) != "" {
		if err := p.linuxUploadUserTOML(o, comm, service); err != nil {
			return err
		}
	}

	// Upload service group key
	if service.ServiceGroupKey != "" {
		err := p.linuxUploadServiceGroupKey(o, comm, service)
		if err != nil {
			return err
		}
	}

	if service.Topology != "" {
		options += fmt.Sprintf(" --topology %s", service.Topology)
	}

	if service.Strategy != "" {
		options += fmt.Sprintf(" --strategy %s", service.Strategy)
	}

	if service.Channel != "" {
		options += fmt.Sprintf(" --channel %s", service.Channel)
	}

	if service.URL != "" {
		options += fmt.Sprintf(" --url %s", service.URL)
	}

	if service.Group != "" {
		options += fmt.Sprintf(" --group %s", service.Group)
	}

	for _, bind := range service.Binds {
		options += fmt.Sprintf(" --bind %s", bind.toBindString())
	}

	// If the svc is already loaded and we require re-loading, unload the service before continuing (don't care
	// about errors at this point, since if it's not already running we just 'hab svc load' anyways)
	if service.Reload || service.Unload {
		o.Output(fmt.Sprintf("Unloading service %s due to reload ...", service.Name))
		_ = p.linuxHabitatServiceUnload(o, comm, service)
	}

	// If the requested service is already loaded, skip re-loading it
	if !service.Unload {
		if err := p.linuxHabitatServiceLoaded(o, comm, service); err != nil {
			return p.runCommand(o, comm, p.linuxGetCommand(fmt.Sprintf("hab svc load %s %s", service.Name, options)))
		}
	}

	return nil
}

// This is a check to see if a habitat svc is already loaded on the machine
func (p *provisioner) linuxHabitatServiceLoaded(o terraform.UIOutput, comm communicator.Communicator, service Service) error {
	return p.runCommand(o, comm, p.linuxGetCommand(fmt.Sprintf("hab svc status %s >/dev/null 2>&1", service.Name)))
}

// This will quietly unload a habitat svc, ignoring any errors
func (p *provisioner) linuxHabitatServiceUnload(o terraform.UIOutput, comm communicator.Communicator, service Service) error {
	return p.runCommand(o, comm, p.linuxGetCommand(fmt.Sprintf("hab svc unload %s > /dev/null 2>&1 ; sleep 3", service.Name)))
}

// In the future we'll remove the dedicated install once the synchronous load feature in hab-sup is
// available. Until then we install here to provide output and a noisy failure mechanism because
// if you install with the pkg load, it occurs asynchronously and fails quietly.
func (p *provisioner) linuxInstallHabitatPackage(o terraform.UIOutput, comm communicator.Communicator, service Service) error {
	var options string

	if service.Channel != "" {
		options += fmt.Sprintf(" --channel %s", service.Channel)
	}

	if service.URL != "" {
		options += fmt.Sprintf(" --url %s", service.URL)
	}

	return p.runCommand(o, comm, p.linuxGetCommand(fmt.Sprintf("hab pkg install %s %s", service.Name, options)))
}

func (p *provisioner) linuxUploadServiceGroupKey(o terraform.UIOutput, comm communicator.Communicator, service Service) error {
	keyName := strings.Split(service.ServiceGroupKey, "\n")[1]
	o.Output("Uploading service group key: " + keyName)
	keyFileName := fmt.Sprintf("%s.box.key", keyName)
	destPath := path.Join("/hab/cache/keys", keyFileName)
	keyContent := strings.NewReader(service.ServiceGroupKey)

	if p.UseSudo {
		tempPath := path.Join("/tmp", keyFileName)
		if err := comm.Upload(tempPath, keyContent); err != nil {
			return err
		}

		return p.runCommand(o, comm, p.linuxGetCommand(fmt.Sprintf("mv %s %s", tempPath, destPath)))
	}

	return comm.Upload(destPath, keyContent)
}

func (p *provisioner) linuxUploadUserTOML(o terraform.UIOutput, comm communicator.Communicator, service Service) error {
	// Create the hab svc directory to lay down the user.toml before loading the service
	o.Output("Uploading user.toml for service: " + service.Name)
	destDir := fmt.Sprintf("/hab/user/%s/config", service.getPackageName(service.Name))
	command := p.linuxGetCommand(fmt.Sprintf("mkdir -p %s", destDir))
	if err := p.runCommand(o, comm, command); err != nil {
		return err
	}

	userToml := strings.NewReader(service.UserTOML)

	if p.UseSudo {
		if err := comm.Upload(fmt.Sprintf("/tmp/user-%s.toml", service.getServiceNameChecksum()), userToml); err != nil {
			return err
		}
		command = p.linuxGetCommand(fmt.Sprintf("mv /tmp/user-%s.toml %s/user.toml", service.getServiceNameChecksum(), destDir))
		return p.runCommand(o, comm, command)
	}

	return comm.Upload(path.Join(destDir, "user.toml"), userToml)
}

func (p *provisioner) linuxGetCommand(command string) string {
	// Always set HAB_NONINTERACTIVE & HAB_NOCOLORING
	env := "env HAB_NONINTERACTIVE=true HAB_NOCOLORING=true"

	// Set license acceptance
	if p.License != "" {
		env += fmt.Sprintf(" HAB_LICENSE=%s", p.License)
	}

	// Set builder auth token
	if p.BuilderAuthToken != "" {
		env += fmt.Sprintf(" HAB_AUTH_TOKEN=%s", p.BuilderAuthToken)
	}

	if p.UseSudo {
		command = fmt.Sprintf("%s sudo -E /bin/bash -c '%s'", env, command)
	} else {
		command = fmt.Sprintf("%s /bin/bash -c '%s'", env, command)
	}

	return command
}
