---
title: "Getting Started"
description: "Get started with ComplyTime in minutes."
lead: "Get up and running with ComplyTime compliance automation tools."
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-07-06T00:00:00+00:00
draft: false
images: []
weight: 200
toc: true
---

## Introduction

ComplyTime is a suite of open source tools designed to automate compliance workflows in cloud native environments. Our engineering-first approach brings compliance into your existing DevSecOps pipeline.

## Architecture Overview

ComplyTime spans two core domains **Definition** and **Measurement** integrated into your Software Development Lifecycle.

{{< theme-image light="/images/complytime-architecture.png" dark="/images/complytime-architecture-dark.png" alt="ComplyTime Architecture Diagram" >}}

- **Definition** — Users author Policies and Controls (with AI assistance via the Gemara MCP Server), which are stored in Git and provide design requirements to the SDLC.
- **Measurement** — `complyctl` and its plugins read those policies, run assessments in the deployment pipeline, and feed findings to enforcement gates, a Collector, and downstream systems like GRC and Observability Platforms.
- **Preventative Enforcement** — An Admission Controller gates the Live Environment in real time, while a failed-job mechanism blocks the pipeline when controls are not met.

## Prerequisites

Before you begin, ensure you have:

- **Git** for cloning repositories

To build from source, you will also need:

- **Go 1.24+**
- **Make**

## Quick Start with complyctl

The fastest way to get started is with `complyctl`, our command-line tool for compliance workflows.

### Installation

**Binary (recommended)**

Download the latest release from the [complyctl releases page](https://github.com/complytime/complyctl/releases). Then verify the release signature using `cosign`:

```bash
cosign verify-blob \
  --certificate complyctl_*_checksums.txt.pem \
  --signature complyctl_*_checksums.txt.sig \
  complyctl_*_checksums.txt \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  --certificate-identity=https://github.com/complytime/complyctl/.github/workflows/release.yml@refs/heads/main
```

**Build from source**

```bash
git clone https://github.com/complytime/complyctl.git
cd complyctl
make build
export PATH="$PWD/bin:$PATH"
```

### Verify Installation

```bash
complyctl version
```

### Install a Scanning Provider

Scanning providers are standalone executables placed in `~/.complytime/providers/`. The filename determines the evaluator ID (e.g. `complyctl-provider-openscap`).

Pre-built Linux binaries are available from the [complytime-providers releases](https://github.com/complytime/complytime-providers/releases/latest) page. To build from source, see the [complytime-providers README](https://github.com/complytime/complytime-providers#install).

Install the provider:

```bash
mkdir -p ~/.complytime/providers
cp complyctl-provider-openscap ~/.complytime/providers/
```

For the OpenSCAP provider, also install the required system packages:

```bash
sudo dnf install -y openscap-scanner scap-security-guide
```

### Your First Compliance Scan

**1. Create a workspace config**

Create `complytime.yaml` in your working directory. This example uses the [CIS Fedora L1 Server](https://quay.io/complytime/policies-cis-fedora-l1-server) policy with the OpenSCAP provider:

```yaml
policies:
  - url: quay.io/complytime/policies-cis-fedora-l1-server:latest
    id: cis-fedora-l1-server

targets:
  - id: my-server
    policies:
      - cis-fedora-l1-server
    variables:
      profile: cis_server_l1
```

The `profile` variable is required by the OpenSCAP provider — it selects which [SSG](https://www.open-scap.org/security-policies/scap-security-guide/) profile to evaluate. List available profiles on your system with:

```bash
oscap info /usr/share/xml/scap/ssg/content/ssg-fedora-ds.xml
```

If the OpenSCAP provider cannot auto-detect the SCAP datastream for your distribution, set `datastream` explicitly:

```yaml
    variables:
      profile: cis_server_l1
      datastream: /usr/share/xml/scap/ssg/content/ssg-cs10-ds.xml
```

See the [OpenSCAP provider configuration](https://github.com/complytime/complytime-providers/blob/main/cmd/openscap-provider/docs/configuration.md) for all target variables and available profiles.

Alternatively, run `complyctl init` for interactive workspace setup.

**2. Fetch policies**

```bash
complyctl get
```

Downloads Gemara policies from the OCI registry into the local cache (`~/.complytime/policies/`). Uses Docker credential helpers — if `docker login` works, `complyctl get` works.

**3. Verify the cache**

```bash
complyctl list
```

**4. Generate assessment configuration**

```bash
complyctl generate --policy-id cis-fedora-l1-server
```

**5. Run the scan**

```bash
# EvaluationLog (default)
complyctl scan --policy-id cis-fedora-l1-server

# Markdown report
complyctl scan --policy-id cis-fedora-l1-server --format pretty

# OSCAL assessment-results
complyctl scan --policy-id cis-fedora-l1-server --format oscal

# SARIF
complyctl scan --policy-id cis-fedora-l1-server --format sarif
```

Output is written to `./.complytime/scan/`.

**6. Check workspace health (optional)**

```bash
complyctl doctor
complyctl providers
```

## Next Steps

- Explore [all ComplyTime projects](/docs/projects/)

