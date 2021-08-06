//
// Windows Habitat Provisioning
//
locals {
  windows-nodes = [{
    id             = vsphere_virtual_machine.windows.id
    name           = vsphere_virtual_machine.windows.name
    address        = vsphere_virtual_machine.windows.default_ip_address
    ctl_secret     = var.habitat.ctl_secret
    gateway_secret = var.habitat.gateway_auth_token
    winrm_username = var.habitat.hab_winrm_username
    winrm_password = var.habitat.hab_winrm_password
    inspec_profile = "windows"
  }]

  windows-node-effortless_user_toml = data.template_file.windows-effortless_user_toml.rendered
}

resource "vsphere_virtual_machine" "windows" {
  //
  // Create from clone
  //
  clone {
    template_uuid = data.vsphere_virtual_machine.windows-template.id
    timeout       = 30
  }

  //
  // Generic VM settings
  //
  num_cpus                    = var.machine.cpus
  memory                      = 8192
  name                        = "windows"
  resource_pool_id            = data.vsphere_resource_pool.pool.id
  datastore_id                = data.vsphere_datastore.datastores[1].id
  folder                      = var.vsphere.folder
  guest_id                    = data.vsphere_virtual_machine.windows-template.guest_id
  scsi_type                   = data.vsphere_virtual_machine.windows-template.scsi_type
  sync_time_with_host         = var.machine.sync_time_with_host
  wait_for_guest_net_routable = var.machine.wait_for_guest_net_routable
  host_system_id              = data.vsphere_host.hosts[1].id
  migrate_wait_timeout        = 60

  //
  // Primary VM disk
  //
  disk {
    label            = "disk0"
    eagerly_scrub    = data.vsphere_virtual_machine.windows-template.disks.0.eagerly_scrub
    thin_provisioned = data.vsphere_virtual_machine.windows-template.disks.0.thin_provisioned
    size             = data.vsphere_virtual_machine.windows-template.disks.0.size
    unit_number      = 0
  }

  //
  // Additional VM disks
  //
  dynamic "disk" {
    for_each = [for d in var.machine.disks : {
      label            = "disk${d.unit_number}"
      unit_number      = d.unit_number
      thin_provisioned = d.thin_provisioned
      size             = d.size_gb
      datastore_id     = data.vsphere_datastore.disks[1].id
    }]

    content {
      label            = disk.value.label
      unit_number      = disk.value.unit_number
      thin_provisioned = disk.value.thin_provisioned
      size             = disk.value.size
      datastore_id     = disk.value.datastore_id
    }
  }

  //
  // NICs
  //
  dynamic "network_interface" {
    for_each = data.vsphere_network.nics

    content {
      network_id = network_interface.value.id
    }
  }

  //
  // Habitat
  //
  provisioner "habitat" {
    license            = var.habitat.license
    version            = var.habitat.version
    peers              = local.supervisor-ring-nodes[*].address
    use_sudo           = var.habitat.use_sudo
    permanent_peer     = false
    listen_ctl         = var.habitat.listen_ctl
    listen_gossip      = var.habitat.listen_gossip
    listen_http        = var.habitat.listen_http
    ring_key           = var.habitat.ring_key_name
    ring_key_content   = var.habitat.ring_key_content
    ctl_secret         = var.habitat.ctl_secret
    url                = var.habitat.url
    channel            = var.habitat.channel
    organization       = var.habitat.organization
    gateway_auth_token = var.habitat.gateway_auth_token
    builder_auth_token = var.habitat.builder_auth_token

    dynamic "service" {
      for_each = [for s in var.habitat.services : {
        name        = s.ident
        topology    = s.topology
        strategy    = s.strategy
        user_toml   = local.windows-node-effortless_user_toml
        channel     = s.channel
        group       = s.group
        url         = s.url
        binds       = s.binds
        reprovision = s.reprovision
      }]

      content {
        name        = service.value.name
        topology    = service.value.topology
        strategy    = service.value.strategy
        user_toml   = service.value.user_toml
        channel     = service.value.channel
        group       = service.value.group
        url         = service.value.url
        binds       = service.value.binds
        reprovision = service.value.reprovision
      }
    }

    connection {
      type        = "winrm"
      https       = true
      insecure    = true
      host        = self.default_ip_address
      user        = var.habitat.hab_winrm_username
      password    = var.habitat.hab_winrm_password
      timeout     = "30m"
    }
  }
}

//
// User toml for klm/effortless (windows)
//
data "template_file" "windows-effortless_user_toml" {
  template = file("effortless/user.tmpl.toml")

  vars = {
    machine_name   = "windows"
    machine_domain = "klmh.co"
  }
}
