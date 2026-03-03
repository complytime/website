---
description: Discover, install, and manage complyctl plugins.
title: Plugin Guide
weight: 40
---
<!-- synced from complytime/complyctl@main (39e8c0849d15) on 2026-02-25T17:21:26Z -->

# Plugin Authoring

Complyctl can be extended to support desired policy engines (PVPs) by the use of plugins.
The plugin acts as the integration between complyctl and the PVPs native interface.
Each plugin is responsible for converting the policy content described in OSCAL into the input format expected by the PVP.
In addition, the plugin converts the raw results provided by the PVP into the schema used by complyctl to generate OSCAL output.

Plugins communicate with complyctl via gRPC and can be authored using any preferred language.
The plugin acts as the gRPC server while the complyctl CLI acts as the client.
When a `complyctl` command is run, it invokes the appropriate method served by the plugin.

Complyctl is built on [compliance-to-policy-go](https://github.com/oscal-compass/compliance-to-policy-go/ which provides a flexible plugin framework for leveraging OSCAL with various PVPs. For developers choosing Golang, the same SDK can be used for plugin authoring.

## Plugin Discovery

Complyctl performs automated plugin discovery using the compliance-to-policy-go [plugin manager](https://github.com/complytime/compliance-to-policy-go/blob/CPLYTM-272/plugin/discovery.go).
Plugins are defined using manifest files placed in the `c2p-plugins` directory.
The plugin manifest is a JSON file that provides metadata about the plugin.
Check the quick start [guide](https://github.com/complytime/complyctl/blob/main/docs/QUICK_START.md) to see an example.

**Note:** the plugin manifest file must have the following syntax for automatic discovery: `c2p-<plugin name>-manifest.json`

### Example Plugin Manifest

```
{
	“id”: “myplugin”,
	“description”: “my example plugin”,
	“version”: “0.1”,
	“type”: [“pvp”],
	“executablePath”: "myplugin" // in relation to the plugin directory
	“sha256”: “23f…” // sha256 of executable
	"configuration": [
      {
        "name": "config_name",
        "description": "Config description",
        "default": "default_value",
        "required": true
      },
	]
}
```

### Directory Naming Conventions

In order to support automated aggregation of output files from multiple plugins the following directory names are expected by complyctl :

**Note:** The `workspace` path will be provided by complyctl via the [configuration](https://github.com/complytime/complyctl/blob/6cf2e92aff852119bba83e579e2c6d8700e4bcec/internal/complytime/plugins.go#L72) and represents the user's desired working directory for all complyctl activities.

- `{workspace}/{plugin name}/results` # files for evidence collection
- `{workspace}/{plugin name}/remediations` # files for automated remediation

### Plugin Selection

Complyctl generates a mapping of plugins to validation components at runtime.
This mapping uses the `title` of the validation component to find a matching plugin with that ID (defined in manifest).

```json
{
	...
	“uuid”: “701c7...”,
	“type”: “validation,
	“title”: “myplugin”, // name must match plugin ID in manifest
}
```

## Example

Below shows an example template for authoring a Golang plugin.

```go

import "github.com/oscal-compass/compliance-to-policy-go/v2/policy"

type PluginServer struct {}

func (s PluginServer) Generate(p policy.Policy) error {

	// PluginServer should implement the Generate() method to provide logic for
	// translating OSCAL to the PVPs expected input format.  Note: this may not be
	// applicable to all PVPs.

}

func (s PluginServer) GetResults(p policy.Policy) (policy.PVPResult, error) {

	// PluginServer should implement the GetResults() method to provide logic to
	// collect results from the PVP for a given policy.  Note: if the PVP requires input
	// from Generate() then the policy input here may be ignored.

}
```
