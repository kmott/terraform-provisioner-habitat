//
// We use Terraform Cloud
//
terraform {
  backend "remote" {
    organization = "klm"

    workspaces {
      name = "terraform-provisioner-habitat"
    }
  }
}

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

//
// This is the random suffix for each deployment
//
resource "random_id" "id" { byte_length = 4 }
