//
// vSphere
//
provider "vsphere" {
  user                 = var.vsphere.user
  password             = var.vsphere.password
  vsphere_server       = var.vsphere.server
  allow_unverified_ssl = true
  version              = "~> 1.14"
}
