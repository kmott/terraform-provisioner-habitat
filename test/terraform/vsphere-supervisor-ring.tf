//
// Linux Habitat Dedicated Supervisor Ring
//
resource "vsphere_virtual_machine" "supervisor-ring" {
  count = 3

  //
  // Create from clone
  //
  clone {
    template_uuid = data.vsphere_virtual_machine.linux-template.id
    timeout       = 30
  }

  //
  // Generic VM settings
  //
  num_cpus                    = var.machine.cpus
  memory                      = var.machine.memory_mb
  name                        = format("sup-ring-%s", count.index+1)
  resource_pool_id            = data.vsphere_resource_pool.pool.id
  datastore_id                = data.vsphere_datastore.datastores[count.index].id
  folder                      = var.vsphere.folder
  guest_id                    = data.vsphere_virtual_machine.linux-template.guest_id
  scsi_type                   = data.vsphere_virtual_machine.linux-template.scsi_type
  sync_time_with_host         = var.machine.sync_time_with_host
  wait_for_guest_net_routable = var.machine.wait_for_guest_net_routable
  host_system_id              = data.vsphere_host.hosts[count.index].id
  migrate_wait_timeout        = 60

  //
  // Primary VM disk
  //
  disk {
    label            = "disk0"
    eagerly_scrub    = data.vsphere_virtual_machine.linux-template.disks.0.eagerly_scrub
    thin_provisioned = data.vsphere_virtual_machine.linux-template.disks.0.thin_provisioned
    size             = data.vsphere_virtual_machine.linux-template.disks.0.size
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
      datastore_id     = data.vsphere_datastore.datastores[count.index].id
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
}

//
// Local vars to setup Supervisor ring data
//
locals {
  supervisor-ring-nodes = [
    {
      id             = vsphere_virtual_machine.supervisor-ring[0].id
      name           = vsphere_virtual_machine.supervisor-ring[0].name
      address        = vsphere_virtual_machine.supervisor-ring[0].default_ip_address
      ctl_secret     = var.habitat.ctl_secret
      gateway_secret = var.habitat.gateway_auth_token
      ssh_username   = var.habitat.hab_ssh_username
      ssh_password   = var.habitat.hab_ssh_password
      inspec_profile = "linux"
    },
    {
      id             = vsphere_virtual_machine.supervisor-ring[1].id
      name           = vsphere_virtual_machine.supervisor-ring[1].name
      address        = vsphere_virtual_machine.supervisor-ring[1].default_ip_address
      ctl_secret     = var.habitat.ctl_secret
      gateway_secret = var.habitat.gateway_auth_token
      ssh_username   = var.habitat.hab_ssh_username
      ssh_password   = var.habitat.hab_ssh_password
      inspec_profile = "linux"
    },
    {
      id             = vsphere_virtual_machine.supervisor-ring[2].id
      name           = vsphere_virtual_machine.supervisor-ring[2].name
      address        = vsphere_virtual_machine.supervisor-ring[2].default_ip_address
      ctl_secret     = var.habitat.ctl_secret
      gateway_secret = var.habitat.gateway_auth_token
      ssh_username   = var.habitat.hab_ssh_username
      ssh_password   = var.habitat.hab_ssh_password
      inspec_profile = "linux"
    }
  ]

  supervisor-ring-effortless-user-toml = data.template_file.supervisor-ring-effortless_user_toml[*].rendered
}

//
// Provision Habitat Supervisor Ring
//
resource "null_resource" "habitat-provisioner" {
  depends_on = [vsphere_virtual_machine.supervisor-ring]
  count      = length(local.supervisor-ring-nodes)

  triggers = {
    cluster_instance_ids = join(",", local.supervisor-ring-nodes[*].id)
    user_toml_contents   = sha256(local.supervisor-ring-effortless-user-toml[count.index])
  }

  provisioner "habitat" {
    license            = var.habitat.license
    version            = var.habitat.version
    peers              = local.supervisor-ring-nodes[*].address
    use_sudo           = var.habitat.use_sudo
    permanent_peer     = var.habitat.permanent_peer
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
        user_toml   = local.supervisor-ring-effortless-user-toml[count.index]
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
        reprovision = (count.index == length(local.supervisor-ring-nodes) - 1) ? true : service.value.reprovision
      }
    }

    connection {
      type        = "ssh"
      host        = local.supervisor-ring-nodes[count.index].address
      user        = var.habitat.hab_ssh_username
      password    = var.habitat.hab_ssh_password
      private_key = file("~/.ssh/klm-id_rsa.pem")
    }
  }
}

//
// User toml for klm/effortless (supervisor-ring)
//
data "template_file" "supervisor-ring-effortless_user_toml" {
  count = 3
  template = file("effortless/user.tmpl.toml")

  vars = {
    machine_name   = vsphere_virtual_machine.supervisor-ring[count.index].name
    machine_domain = "klmh.co"
  }
}
