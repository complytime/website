---
title: "Getting Started"
description: "Get started with ComplyTime in minutes."
lead: "Get up and running with ComplyTime compliance automation tools."
date: 2024-01-01T00:00:00+00:00
lastmod: 2024-12-24T00:00:00+00:00
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

- **[Git](https://git-scm.com/)** for cloning repositories
- **[Sigstore Cosign](https://github.com/sigstore/cosign)** for OCI registry object validation

To build from source, you will also need:

- **[Go](https://go.dev/) 1.24+**
- **[Make](https://www.gnu.org/software/make/)**

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

> **Note:** ComplyTime uses two separate `.complytime` directories with different scopes:
> `~/.complytime/` (under your home directory) is a **global** cache for providers and downloaded policies,
> while `./.complytime/` (in your current working directory) holds **per-workspace** state such as `complytime.yaml` and scan output.

Scanning providers are standalone executables placed in `~/.complytime/providers/`. The filename determines the evaluator ID (e.g. `complyctl-provider-openscap`).

```bash
mkdir -p ~/.complytime/providers
cp bin/complyctl-provider-openscap ~/.complytime/providers/
```

For the OpenSCAP provider, also install the required system packages:

- `openscap-scanner`
- `scap-security-guide`

### Your First Compliance Scan

**1. Initialize a workspace**

```bash
complyctl init
```

Creates a `complytime.yaml` workspace config. If one already exists, it validates and runs `get` automatically.

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
complyctl generate --policy-id <policy-id>
```

**5. Run the scan**

```bash
# EvaluationLog (default)
complyctl scan --policy-id <policy-id>

# Markdown report
complyctl scan --policy-id <policy-id> --format pretty

# OSCAL assessment-results
complyctl scan --policy-id <policy-id> --format oscal

# SARIF
complyctl scan --policy-id <policy-id> --format sarif
```

Output is written to `./.complytime/scan/`.

**6. Check workspace health (optional)**

```bash
complyctl doctor
complyctl providers
```

## Next Steps

- Explore [all ComplyTime projects](/docs/projects/)

