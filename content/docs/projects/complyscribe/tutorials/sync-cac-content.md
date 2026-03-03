---
description: Synchronize Compliance-as-Code content with complyscribe.
title: Sync CaC Content
weight: 43
---
<!-- synced from complytime/complyscribe@main (7eb115094f17) on 2026-02-25T17:21:26Z -->

# The complyscribe command line sync-cac-content Tutorial

This tutorial provides how to use `complyscribe sync-cac-content` transform [cac-content](https://github.com/ComplianceAsCode/content) to OSCAL models.
This command has three sub-commands `catalog`, `profile` and `component-definition`

> **WARNING:** There is a sequential order when transformed, first Catalog, then Profile, last Component Definition.
> Because Profile depends on Catalog, and Component Definition depends on Profile.

## catalog

This command is to generate OSCAL Catalog according to CaC content policy 

### 1. Prerequisites

- Initialize the [complyscribe workspace](../tutorials/github.md#3-initialize-complyscribe-workspace) if you do not have one.

- Clone the [cac-content repository](https://github.com/ComplianceAsCode/content).

### 2. Run the CLI sync-cac-content catalog

A real world example, if we want to transform [cis_rhel8](https://github.com/ComplianceAsCode/content/blob/master/controls/cis_rhel8.yml)
to OSCAL Catalog, we run command like below,`cac-policy-id` is [control file id](https://github.com/ComplianceAsCode/content/blob/master/controls/cis_rhel8.yml#L4),
`oscal-catalog` is OSCAL Catalog directory name we will use when generating the OSCAL Catalog.

```shell
poetry run complyscribe sync-cac-content catalog \
--dry-run \
--repo-path $complyscribe_workspace_root_dir \
--committer-email tester@redhat.com \
--committer-name tester \
--branch main \
--cac-policy-id cis_rhel8 \
--oscal-catalog cis_rhel8 \
--cac-content-root $cac_content_root_dir
```

After successfully running above command, will generate [catalogs/cis_rhel8/catalog.json](https://github.com/ComplianceAsCode/oscal-content/blob/main/catalogs/cis_rhel8/catalog.json)

For more details about these options and additional flags, you can use the `--help` flag:
`poetry run complyscribe sync-cac-content catalog --help`
This will display a full list of available options and their descriptions.

After running the CLI with the right options, you would successfully generate an OSCAL Catalog under
`$complyscribe_workspace_root_dir/catalogs`.


## profile

This command is to generate OSCAL Profile according to content policy 

### 1. Prerequisites

- Initialize the [complyscribe workspace](../tutorials/github.md#3-initialize-complyscribe-workspace) if you do not have one.

- Clone the [cac-content repository](https://github.com/ComplianceAsCode/content).

### 2. Run the CLI sync-cac-content profile

A real world example, if we want to transform [rhel8 product](https://github.com/ComplianceAsCode/content/tree/master/products/rhel8)
that using [cis_rhel8 control file](https://github.com/ComplianceAsCode/content/blob/master/controls/cis_rhel8.yml) to OSCAL Profile,
we run command like below, `product` is [product name](https://github.com/ComplianceAsCode/content/blob/master/products/rhel8/product.yml#L1), 
`oscal-catalog` is OSCAL [catalog directory name](https://github.com/ComplianceAsCode/oscal-content/tree/main/catalogs/cis_rhel8),
`cac-policy-id` is [control file id](https://github.com/ComplianceAsCode/content/blob/master/controls/cis_rhel8.yml#L4)

```shell
poetry run complyscribe sync-cac-content profile \
--dry-run \
--repo-path $complyscribe_workspace_root_dir \
--committer-email tester@redhat.com \
--committer-name tester \
--branch main \
--cac-content-root $cac_content_root_dir \
--product rhel8 \
--oscal-catalog cis_rhel8 \
--cac-policy-id cis_rhel8
```

After successfully running above command, you will generate four OSCAL
Profiles([rhel8-cis_rhel8-l1_server](https://github.com/ComplianceAsCode/oscal-content/blob/main/profiles/rhel8-cis_rhel8-l1_server/profile.json)
,[rhel8-cis_rhel8-l2_server](https://github.com/ComplianceAsCode/oscal-content/blob/main/profiles/rhel8-cis_rhel8-l2_server/profile.json),
[rhel8-cis_rhel8-l1_workstation](https://github.com/ComplianceAsCode/oscal-content/blob/main/profiles/rhel8-cis_rhel8-l1_workstation/profile.json),
[rhel8-cis_rhel8-l2_workstation](https://github.com/ComplianceAsCode/oscal-content/blob/main/profiles/rhel8-cis_rhel8-l2_workstation/profile.json)), 
every [level](https://github.com/ComplianceAsCode/content/blob/master/controls/cis_rhel8.yml#L8) has its own Profile.

For more details about these options and additional flags, you can use the `--help` flag:
`poetry run complyscribe sync-cac-content profile --help`
This will display a full list of available options and their descriptions.

After running the CLI with the right options, you would successfully generate an OSCAL Profile
under `$complyscribe_workspace_root_dir/profiles/$product_$cac-policy-id_$level`.

## component-definition

This command creates OSCAL Component Definitions by transforming CaC content control files.

The CLI performs the following transformations:

- Populate CaC product information to OSCAL component title and description
- Ensure OSCAL component control mappings are populated with rule and rule parameter data from CaC control files
- Create a validation component from SSG rules to check mappings
- Ensure OSCAL Component Definition implemented requirements are populated from control notes in the control file
- Ensure implementation status of an implemented requirement in OSCAL Component Definitions are populated with the status from CaC control files

### 1. Prerequisites

- Initialize the [complyscribe workspace](../tutorials/github.md#3-initialize-complyscribe-workspace).

- Clone the [cac-content repository](https://github.com/ComplianceAsCode/content).

### 2. Run the CLI sync-cac-content component-definition

A real world example. If we want to transform [cis_server_l1.profile](https://github.com/ComplianceAsCode/content/blob/master/products/rhel8/profiles/cis_server_l1.profile)
to an OSCAL Component Definition, we run command like below. `product` is [product name](https://github.com/ComplianceAsCode/content/blob/master/products/rhel8/product.yml#L1),
`cac-profile` is [CaC content profile file name](https://github.com/ComplianceAsCode/content/blob/master/products/rhel8/profiles/cis_server_l1.profile) you need transform,
`oscal-profile` is [OSCAL profile directory name](https://github.com/ComplianceAsCode/oscal-content/blob/main/profiles/rhel8-cis_rhel8-l1_server/profile.json) corresponding 
to CaC content profile, `component-definition-type` is [a category describing the purpose of the component](https://pages.nist.gov/OSCAL-Reference/models/v1.1.3/component-definition/json-reference/#/component-definition/components/type).

```shell
poetry run complyscribe sync-cac-content component-definition \
--dry-run \
--repo-path $complyscribe_workspace_root_dir \
--committer-email tester@redhat.com \
--committer-name tester \
--branch main \
--cac-content-root $cac_content_root_dir \
--product rhel8 \
--component-definition-type software \
--oscal-profile rhel8-cis_rhel8-l1_server \
--cac-profile cis_server_l1
```

After successfully running above command, will generate an OSCAL [Component Definition](https://github.com/ComplianceAsCode/oscal-content/blob/main/component-definitions/rhel8/rhel8-cis_rhel8-l1_server/component-definition.json) 

For more details about these options and additional flags, you can use the `--help` flag:
`poetry run complyscribe sync-cac-content component-definition --help`
This will display a full list of available options and their descriptions.

After running the CLI with the right options, you would successfully generate an OSCAL Component Definition
under $complyscribe_workspace_root_dir/component-definitions/$product_name/$OSCAL-profile-name.
