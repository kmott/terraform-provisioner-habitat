//
// Host(s)
//
data "vsphere_host" "hosts" {
  count = 3
  name          = format("172.16.46.20%s", count.index+1)
  datacenter_id = data.vsphere_datacenter.dc.id
}

//
// Datacenter
//
data "vsphere_datacenter" "dc" {
  name = var.vsphere.datacenter
}

//
// Pool
//
data "vsphere_resource_pool" "pool" {
  name          = "${var.vsphere.cluster}/Resources/${var.vsphere.pool}"
  datacenter_id = data.vsphere_datacenter.dc.id
}

//
// Template
//
data "vsphere_virtual_machine" "linux-template" {
  name          = var.machine.linux_template
  datacenter_id = data.vsphere_datacenter.dc.id
}

data "vsphere_virtual_machine" "windows-template" {
  name          = var.machine.windows_template
  datacenter_id = data.vsphere_datacenter.dc.id
}

//
// Disks
//
data "vsphere_datastore" "datastores" {
  count = 3
  name          = format("klm-vms-0%s-local", count.index+1)
  datacenter_id = data.vsphere_datacenter.dc.id
}

//
// Additional disks
//
data "vsphere_datastore" "disks" {
  count = 3
  name          = format("klm-vms-0%s-local", count.index+1)
  datacenter_id = data.vsphere_datacenter.dc.id
}

//
// NICs
//
data "vsphere_network" "nics" {
  count         = length(var.machine.nics)
  name          = var.machine.nics[count.index]
  datacenter_id = data.vsphere_datacenter.dc.id
}
