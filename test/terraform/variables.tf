//
// vSphere variables
//
variable "vsphere" {
  type = object({
    user       = string
    password   = string
    server     = string
    datacenter = string
    cluster    = string
    pool       = string
    folder     = string
  })
}

//
// Machine variable
//
variable "machine" {
  type = object({
    name      = string
    domain    = string
    cpus      = number
    memory_mb = number
    linux_template  = string
    windows_template  = string
    nics      = list(string)
    disks = list(object({
      unit_number      = number
      size_gb          = number
      thin_provisioned = bool
    }))
    wait_for_guest_net_routable = bool
    sync_time_with_host         = bool
  })
}

//
// Habitat variable
//
variable "habitat" {
  type = object({
    license            = string
    version            = string
    peers              = list(string)
    hab_ssh_username   = string
    hab_ssh_password   = string
    hab_winrm_username   = string
    hab_winrm_password   = string
    use_sudo           = bool
    permanent_peer     = bool
    listen_ctl         = string
    listen_gossip      = string
    listen_http        = string
    ring_key_name      = string
    ring_key_content   = string
    ctl_secret         = string
    url                = string
    channel            = string
    organization       = string
    gateway_auth_token = string
    builder_auth_token = string
    services = list(object({
      ident              = string
      topology           = string
      strategy           = string
      user_toml_contents = string
      channel            = string
      group              = string
      url                = string
      binds              = list(string)
      reprovision        = bool
    }))
  })
}
