---
description: Synchronize OSCAL content with complyscribe.
title: Sync OSCAL Content
weight: 44
---
<!-- synced from complytime/complyscribe@main (7eb115094f17) on 2026-02-25T17:21:26Z -->

# The complyscribe command line sync-oscal-content Tutorial

This tutorial provides how to use `complyscribe sync-oscal-content` sync OSCAL models to [cac-content](https://github.com/ComplianceAsCode/content).

Currently, this command has three sub-command: `component-definition` and `profile` and `catalog`

## component-definition

This command is to sync OSCAL Component Definition information to CaC content side.

The CLI performs the following sync:

- Sync OSCAL Component Definition parameters/rules changes to CaC content profile file
- Sync OSCAL Component Definition parameters/rules changes to CaC content control file
- Add a hint comment to the control file when a missing rule is found in the CaC content repo.
- Sync OSCAL Component Definition control status changes to CaC content control file. Since status mapping between
cac and OSCAL is many-to-many relationship, if status can not be determined when sync, then add a comment to let user
decide. Discussion detail in [doc](https://github.com/complytime/complyscribe/discussions/511)
- Add new option to cac var file when found variable exists but missing the option we sync.
- Sync OSCAL Component Definition statements field to CaC control notes field

### 1. Prerequisites

- Initialize the [complyscribe workspace](../tutorials/github.md#3-initialize-complyscribe-workspace).

- Clone the [cac-content repository](https://github.com/ComplianceAsCode/content).

- Has an OSCAL Component Definition file, (transformed from CaC content using `sync-cac-content component-definition` cmd)

### 2. Run the CLI sync-oscal-content component-definition
```shell
poetry run complyscribe sync-oscal-content component-definition \ 
--branch main \
--cac-content-root $cac_content_root_dir \
--committer-name tester \
--committer-email tester@redhat.com \
--dry-run \
--repo-path $complyscribe_workspace_root_dir \
--product $product-name \
--oscal-profile $oscal-profile-name
```

For more details about these options and additional flags, you can use the --help flag:
`poetry run complyscribe sync-oscal-content component-definition --help`
This will display a full list of available options and their descriptions.


## profile

This command is to sync OSCAL Profile information to CaC content side.

The CLI performs the following sync:

- Sync OSCAL Profile control levels change to CaC control files

### 1. Prerequisites

- Initialize the [complyscribe workspace](../tutorials/github.md#3-initialize-complyscribe-workspace).

- Clone the [cac-content repository](https://github.com/ComplianceAsCode/content).

- Have OSCAL Profile file, (transformed from CaC content using `sync-cac-content profile` cmd)

### 2. Run the CLI sync-oscal-content profile

```shell
poetry run complyscribe sync-oscal-content profile \
--dry-run \
--repo-path $complyscribe_workspace_root_dir \
--committer-email tester@redhat.com \
--committer-name tester \
--branch main \
--cac-content-root $cac_content_root_dir \
--cac-policy-id cis_rhel8 \
--product rhel8
```

For more details about these options and additional flags, you can use the --help flag:
`poetry run complyscribe sync-oscal-content profile --help`
This will display a full list of available options and their descriptions.

## catalog

This command is to sync OSCAL Catalog information to CaC content side.

The CLI performs the following sync:

- Sync OSCAL Catalog control parts field change to CaC control files control description field

### 1. Prerequisites

- Initialize the [complyscribe workspace](../tutorials/github.md#3-initialize-complyscribe-workspace).

- Clone the [cac-content repository](https://github.com/ComplianceAsCode/content).

- An OSCAL Catalog file, (transformed from CaC content using `sync-cac-content catalog` cmd)

### 2. Run the CLI sync-oscal-content catalog
```shell
poetry run complyscribe sync-oscal-content catalog \
--cac-policy-id nist_ocp4 \
--cac-content-root $cac_content_root_dir \
--repo-path $complyscribe_workspace_root_dir \
--committer-name tester \
--committer-email tester@redhat.com \
--branch main \
--dry-run
```

For more details about these options and additional flags, you can use the --help flag:
`poetry run complyscribe sync-oscal-content catalog --help`
This will display a full list of available options and their descriptions.