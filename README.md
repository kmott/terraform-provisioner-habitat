# Summary

The purpose of the provisioner is to provide an easy method for Chef [Habitat](https://habitat.sh) provisioning to 
Linux or Windows machines via Terraform.

It allows loading Habitat specific services and creating a dedicated supervisor ring, and joining other machines to that 
ring via `peers`.

# Habitat Versions

The currently supported version of Chef Habitat is >= v1.5.X.

# Installation

Note that although `terraform-provisioner-habitat` is in the Terraform registry, it cannot be installed using a `module` 
terraform stanza, as such a configuration will not cause terraform to download the `terraform-provisioner-habitat` binary.

- Download a pre-built binary release from [GitHub Releases](https://github.com/kmott/terraform-provisioner-habitat/releases) page
- Make sure the filename matches: `terraform-provisioner-habitat_v<version>`
- Place the file in `~/.terraform.d/plugins/` directory

# Examples

For a simple example, review [vsphere-linux.tf](test/terraform/vsphere-linux.tf) which deploys a simple VM,
configures Habitat, and loads one service (`klm/effortless`).

For a more detailed example, review [vsphere-supervisor-ring.tf](test/terraform/vsphere-supervisor-ring.tf), which
deploys 3 VMs and then uses a `null_resource` to deploy Habitat to each and join them together as permanent-peers.  Additionally,
the [vsphere-linux.tf](test/terraform/vsphere-linux.tf) and [vsphere-windows.tf](test/terraform/vsphere-windows.tf) machines are
created, and join the Supervisor Permanent ring peers, and at least one service is loaded on each VM (`klm/effortless`).

# Config Reference

There are 2 configuration levels, `supervisor` and `service`.  Configuration placed directly within the `provisioner` block
are supervisor configurations, and a provisioner can define zero or more services to run, and each service will have a 
`service` block within the `provisioner`.  A `service` block can also contain zero or more `bind` blocks to create 
service group bindings.

## Supervisor Arguments
| Name | Type | Required? | Description | Default |
|------|------|-----------|-------------|---------|
| `version` | `string`  | no   | Habitat version to install  | `latest` |
| `license` | `string`  | yes  | License acceptance (`accept` or `accept-no-persist`)  | - |
| `auto_update` | `bool`  | no   | If set to `true`, supervisor will auto-update itself from the specified `channel` | - |
| `http_disable` | `bool`  | no   | If set to `true`, disables the supervisor HTTP listener entirely | - |
| `peers` | `list(string)`  | no   | A list of IP or FQDN's of other supervisor instance(s) to peer with | - |
| `service_type` | `string`  | no   | Method used to run the Habitat supervisor.  Valid options are `unmanaged` and `systemd` | `systemd` |
| `service_name` | `string`  | no   | The name of the Habitat supervisor service, if using an init system such as `systemd` | `hab-supervisor` |
| `use_sudo` | `bool`  | no   | Use `sudo` when executing remote commands.  Required when the user specified in the `connection` block is not `root` | `true` |
| `permanent_peer` | `bool`  | no   | Marks this supervisor as a permanent peer | `false` |
| `listen_ctl` | `string`  | no   | The listen address for the control gateway system | `127.0.0.1:9632` |
| `listen_gossip` | `string`  | no   | The listen address for the gossip system | `0.0.0.0:9638` |
| `listen_http` | `string`  | no   | The listen address for the HTTP gateway | `0.0.0.0:9631` |
| `ring_key` | `string`  | no   | The name of the ring key for encrypting gossip ring communication | - |
| `ring_key_content` | `string`  | no   | The ring key content.  Easiest to source from a file (eg `ring_key_content = "${file("conf/foo-123456789.sym.key")}"`) | - |
| `ctl_secret` | `string`  | no   | Specify a secret to use (from `hab sup secret generate`) for control gateway communication between hab client(s) and the supervisor | - |
| `url` | `string`  | no   | The URL of a Builder service to download packages and receive updates from | `https://bldr.habitat.sh` |
| `channel` | `string`  | no   | The release channel in the Builder service to use | `stable` |
| `events` | `string`  | no   | Name of the service group running a Habitat EventSrv to forward Supervisor and service event data to | - |
| `organization` | `string`  | no   | The organization that the Supervisor and it's subsequent services are part of | `default` |
| `gateway_auth_token` | `string` | no   | The http gateway authorization token | - |
| `builder_auth_token` | `string` | no   | The builder authorization token when using a private origin | - |
| `service` | `list(object)` | no   | One or more `service` blocks to start Habitat services after installation | - |
| `event_stream` | `object` | no   | One `event_stream` block to configure the supervisor with during startup | - |

## `service` Arguments
| Name | Type | Required? | Description | Default |
|------|------|-----------|-------------|---------|
| `name` | `string` | yes | The Habitat package identifier of the service to run (e.g., `core/haproxy` or `core/redis/3.2.4/20171002182640`) | - |
| `binds` | `list(string)` | no | An list of bind specifications (ie `binds = ["backend:nginx.default"]`) | - |
| `bind` | `block` | no | An alternative way of declaring binds.  This method can be easier to deal with when populating values from other values or variable inputs without having to do string interpolation. The example below is equivalent to `binds = ["backend:nginx.default"]`: | - |
| `topology` | `string` | no | Topology to start service in. Possible values `standalone` or `leader` | `standalone` |
| `strategy` | `string` | no | Update strategy to use. Possible values `at-once`, `rolling` or `none` | `none` |
| `user_toml` | `string` | no | TOML formatted user configuration for the service. Easiest to source from a file (eg `user_toml = "${file("conf/redis.toml")}"`) | - |
| `channel` | `string` | no | The release channel in the Builder service to use | `stable` |
| `group` | `string` | no | The service group to join | `default` |
| `url` | `string` | no | The URL of a Builder service to download packages and receive updates from | `https://bldr.habitat.sh` |
| `application` | `string` | no | The application name | - |
| `environment` | `string` | no | The environment name | - |
| `service_key` | `string` | no | The key content of a service private key, if using service group encryption.  Easiest to source from a file (eg `service_key = "${file("conf/redis.default@org-123456789.box.key")}"`) | - |
| `reload` | `bool` | no | When set to `true`, unloads a service before `hab svc load` (use for cases where you need to manually re-load a service)  | - |
| `unload` | `bool` | no | When set to `true`, ensures a service is unloaded from the supervisor (mutually exclusive with `reload`) | - |

```hcl
# Alternate `bind` block definition for service group bindings
bind {
  alias = "backend"
  service = "nginx"
  group = "linux"
}
```

## `event_stream` Arguments
| Name | Type | Required? | Description | Default |
|------|------|-----------|-------------|---------|
| `application` | `string`  | yes | The name of the application for event stream purposes, attached to all events generated by this Supervisor | - |
| `environment` | `string`  | yes | The name of the environment for event stream purposes, attached to all events generated by this Supervisor | - |
| `connect_timeout` | `int`  | no | Event stream connection timeout before exiting the Supervisor, set to '0' to immediately start the Supervisor and continue running regardless of the initial connection status | 0 |
| `meta` | `map[string]string`  | no | An arbitrary key-value pair to add to each event generated by this Supervisor | - |
| `server_certificate` | `string`  | no | The path to Chef Automate's event stream certificate used to establish a TLS connection, should be in PEM format | - |
| `site` | `string`  | no | The name of the site where this Supervisor is running for event stream purposes | - |
| `token` | `string`  | yes | The authentication token for connecting the event stream to Chef Automate | - |
| `url` | `string`  | yes | The event stream connection url used to send events to Chef Automate, enables the event stream | - |

# Building

Ensure you have the go toolchain installed, checkout the source code, and run the following command:

`kmott@kmott-sabayon ~/terraform-provisioner-habitat $ make build`

# Testing

There are two main levels of testing--terraform acceptance testing, and integration testing.  The acceptance testing follows
the standard `TF_ACC=1` testing methodology, and can be invoked by running:

`kmott@kmott-sabayon ~/terraform-provisioner-habitat $ make test-acceptance`

Integration testing is a bit more involved, deploying several virtual machines to a vCenter cluster, and invoking 
Terratest + Chef Inspec tests against them.

# Future Considerations

## Windows Support ([Issue #1](https://github.com/kmott/terraform-provisioner-habitat/issues/1))

Unfortunately, the Windows support is a bit weak currently.  Most of the code is in-place, but not much has been done to
test and verify the integration, PRs welcome.

# Biome / CINC Support ([Issue #2](https://github.com/kmott/terraform-provisioner-habitat/issues/1))

At some point in the future, pulling in support for provisioning [Biome](https://biome.sh/en/) and/or 
[CINC Packager](https://cinc.sh/download/#cinc-packager) (when it becomes available) should be fairly trivial.
