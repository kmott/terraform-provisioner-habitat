package habitat

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/terraform"
)

func (p *provisioner) windowsInstallHabitat(o terraform.UIOutput, comm communicator.Communicator) error {
	var err error

	// Setup TLS v1.2 support
	err = p.runCommand(o, comm, p.windowsGetCommand(`[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12`))
	if err != nil {
		return err
	}

	// Set license metadata
	if p.License != "" {
		err = p.runCommand(o, comm, p.windowsGetCommand(fmt.Sprintf(`[System.Environment]::SetEnvironmentVariable(\"HAB_LICENSE\", \"%s\", [System.EnvironmentVariableTarget]::Machine)`, p.License)))
		if err != nil {
			return err
		}

		err = p.runCommand(o, comm, p.windowsGetCommand(fmt.Sprintf(`[System.Environment]::SetEnvironmentVariable(\"HAB_LICENSE\", \"%s\", [System.EnvironmentVariableTarget]::Process)`, p.License)))
		if err != nil {
			return err
		}

		err = p.runCommand(o, comm, p.windowsGetCommand(fmt.Sprintf(`[System.Environment]::SetEnvironmentVariable(\"HAB_LICENSE\", \"%s\", [System.EnvironmentVariableTarget]::User)`, p.License)))
		if err != nil {
			return err
		}
	}

	// Download habitat
	err = p.runCommand(o, comm, p.windowsGetCommand(`irm https://raw.githubusercontent.com/habitat-sh/habitat/master/components/hab/install.ps1 > C:\Windows\TEMP\install.ps1`))
	if err != nil {
		return err
	}

	// Install habitat
	err = p.runCommand(o, comm, p.windowsRunFileWithArgs("C:\\Windows\\TEMP\\install.ps1", fmt.Sprintf("-Version %s", p.Version)))
	if err != nil {
		return err
	}

	// Install version dependent hab-sup
	if p.Version != "latest" {
		err = p.runCommand(o, comm, p.windowsGetCommand(fmt.Sprintf(`hab pkg install core/hab-sup/%s`, p.Version)))
		if err != nil {
			return err
		}
	} else {
		err = p.runCommand(o, comm, p.windowsGetCommand(`hab pkg install core/hab-sup`))
		if err != nil {
			return err
		}
	}

	// Install habitat service pkg (which automatically invokes the install hook for setting up the Windows service)
	err = p.runCommand(o, comm, p.windowsGetCommand(`hab pkg install core/windows-service`))
	if err != nil {
		return err
	}

	// Setup Windows firewall
	err = p.runCommand(o, comm, p.windowsGetCommand(`New-NetFirewallRule -DisplayName \"Habitat TCP\" -Direction Inbound -Action Allow -Protocol TCP -LocalPort 9631,9638`))
	if err != nil {
		return err
	}

	err = p.runCommand(o, comm, p.windowsGetCommand(`New-NetFirewallRule -DisplayName \"Habitat UDP\" -Direction Inbound -Action Allow -Protocol UDP -LocalPort 9638`))
	if err != nil {
		return err
	}

	// Set ctl gateway secret token
	if p.GatewayAuthToken != "" {
		err = p.runCommand(o, comm, p.windowsGetCommand(fmt.Sprintf(`[System.Environment]::SetEnvironmentVariable(\"HAB_SUP_GATEWAY_AUTH_TOKEN\", \"%s\", [System.EnvironmentVariableTarget]::Machine)`, p.GatewayAuthToken)))
		if err != nil {
			return err
		}

		err = p.runCommand(o, comm, p.windowsGetCommand(fmt.Sprintf(`[System.Environment]::SetEnvironmentVariable(\"HAB_SUP_GATEWAY_AUTH_TOKEN\", \"%s\", [System.EnvironmentVariableTarget]::Process)`, p.GatewayAuthToken)))
		if err != nil {
			return err
		}

		err = p.runCommand(o, comm, p.windowsGetCommand(fmt.Sprintf(`[System.Environment]::SetEnvironmentVariable(\"HAB_SUP_GATEWAY_AUTH_TOKEN\", \"%s\", [System.EnvironmentVariableTarget]::User)`, p.GatewayAuthToken)))
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *provisioner) windowsStartHabitat(o terraform.UIOutput, comm communicator.Communicator) error {
	var err error
	var content string
	var options string

	if p.PermanentPeer {
		options += " --permanent-peer"
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

	content += "$svcPath = Join-Path $env:SystemDrive \"hab\\svc\\windows-service\";"
	content += "[xml]$configXml = Get-Content (Join-Path $svcPath HabService.dll.config);"
	content += fmt.Sprintf("$configXml.configuration.appSettings.ChildNodes[\"2\"].value = '%s';", options)
	content += "$configXml.Save((Join-Path $svcPath HabService.dll.config));"

	err = p.runCommand(o, comm, p.windowsGetCommand(content))
	if err != nil {
		return err
	}

	return p.runCommand(o, comm, p.windowsGetCommand("Restart-Service Habitat"))
}

func (p *provisioner) windowsUploadRingKey(o terraform.UIOutput, comm communicator.Communicator) error {
	p.RingKeyContent = strings.ReplaceAll(p.RingKeyContent, "\n", "`n")
	return p.runCommand(o, comm, fmt.Sprintf(`powershell.exe -Command echo %s | hab ring key import`, p.RingKeyContent))
}

func (p *provisioner) windowsUploadCtlSecret(o terraform.UIOutput, comm communicator.Communicator) error {
	destination := "C:\\hab\\sup\\default"
	err := p.runCommand(o, comm, p.windowsGetCommand(fmt.Sprintf("mkdir %s | out-null", destination)))
	if err != nil {
		return err
	}

	return comm.Upload(fmt.Sprintf("%s\\CTL_SECRET", destination), strings.NewReader(p.CtlSecret))
}

func (p *provisioner) windowsStartHabitatService(o terraform.UIOutput, comm communicator.Communicator, service Service) error {
	var options string

	if err := p.windowsInstallHabitatPackage(o, comm, service); err != nil {
		return err
	}

	if strings.TrimSpace(service.UserTOML) != "" {
		if err := p.windowsUploadUserTOML(o, comm, service); err != nil {
			return err
		}
	}

	if service.ServiceGroupKey != "" {
		if err := p.windowsUploadServiceGroupKey(o, comm, service); err != nil {
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
		_ = p.windowsHabitatServiceUnload(o, comm, service)
	}

	// If the requested service is already loaded, skip re-loading it
	if !service.Unload {
		if err := p.windowsHabitatServiceLoaded(o, comm, service); err != nil {
			return p.runCommand(o, comm, p.windowsGetCommand(fmt.Sprintf("hab svc load %s %s", service.Name, options)))
		}
	}

	return nil
}

// This is a check to see if a habitat svc is already loaded on the machine
func (p *provisioner) windowsHabitatServiceUnload(o terraform.UIOutput, comm communicator.Communicator, service Service) error {
	return p.runCommand(o, comm, p.windowsGetCommand(fmt.Sprintf("hab svc unload %s 2>&1 | out-null ; start-sleep -s 3", service.Name)))
}

func (p *provisioner) windowsHabitatServiceLoaded(o terraform.UIOutput, comm communicator.Communicator, service Service) error {
	return p.runCommand(o, comm, p.windowsGetCommand(fmt.Sprintf("hab svc status %s 2>&1 | out-null", service.Name)))
}

func (p *provisioner) windowsInstallHabitatPackage(o terraform.UIOutput, comm communicator.Communicator, service Service) error {
	var options string

	if service.Channel != "" {
		options += fmt.Sprintf(" --channel %s", service.Channel)
	}

	if service.URL != "" {
		options += fmt.Sprintf(" --url %s", service.URL)
	}

	return p.runCommand(o, comm, p.windowsGetCommand(fmt.Sprintf("hab pkg install %s %s", service.Name, options)))
}

func (p *provisioner) windowsUploadServiceGroupKey(o terraform.UIOutput, comm communicator.Communicator, service Service) error {
	keyName := strings.Split(service.ServiceGroupKey, "\n")[1]
	o.Output("Uploading service group key: " + keyName)
	keyFileName := fmt.Sprintf("%s.box.key", keyName)
	keyContent := strings.NewReader(service.ServiceGroupKey)

	return comm.Upload(fmt.Sprintf("C:\\hab\\cache\\keys\\%s", keyFileName), keyContent)
}

func (p *provisioner) windowsUploadUserTOML(o terraform.UIOutput, comm communicator.Communicator, service Service) error {
	// Create the hab svc directory to lay down the user.toml before loading the service
	o.Output("Uploading user.toml for service: " + service.Name)
	svcName := service.getPackageName(service.Name)
	destDir := fmt.Sprintf("C:\\hab\\user\\%s\\config", svcName)
	command := fmt.Sprintf("mkdir %s | out-null", destDir)

	if err := p.runCommand(o, comm, p.windowsGetCommand(command)); err != nil {
		return err
	}

	userToml := strings.NewReader(service.UserTOML)

	return comm.Upload(fmt.Sprintf("%s\\user.toml", destDir), userToml)
}

func (p *provisioner) windowsGetCommand(command string) string {
	// Always set HAB_NONINTERACTIVE & HAB_NOCOLORING
	env := `$Env:HAB_NONINTERACTIVE=\"true\"; $Env:HAB_NOCOLORING=\"true\"; `

	// Set license acceptance
	if p.License != "" {
		env += fmt.Sprintf(`$Env:HAB_LICENSE=\"%s\"; `, p.License)
	}

	// Set builder auth token
	if p.BuilderAuthToken != "" {
		env += fmt.Sprintf(`$Env:HAB_AUTH_TOKEN=\"%s\"; `, p.BuilderAuthToken)
	}

	return fmt.Sprintf(`powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "%s%s"`, env, command)
}

func (p *provisioner) windowsRunFileWithArgs(file, args string) string {
	return fmt.Sprintf(`powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%s" %s`, file, args)
}
