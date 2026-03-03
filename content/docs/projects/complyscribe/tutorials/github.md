---
description: Using complyscribe with GitHub Actions.
title: GitHub Integration
weight: 42
---
<!-- synced from complytime/complyscribe@main (7eb115094f17) on 2026-02-25T17:21:26Z -->

# GitHub Tutorial

This tutorial provides an introduction to using `complyscribe` with GitHub.  We will be using a single GitHub repository for our trestle authoring workspace and executing the `complyscribe` commands as GitHub actions.  Note, each repo is intended to support authoring a single OSCAL model type (SSP, component definition, etc.).  If authoring more than one OSCAL model type, then a dedicated repository should be used for each model.


### 1. Prerequisites

Before moving on, please ensure the following is completed:

1. Create a new (or use an existing) empty GitHub repository
2. Clone the repo to a local workstation
3. Install complyscribe
    * Option 1: Clone the [complyscribe](https://github.com/complytime/complyscribe/tree/main) repo to a local workstation and run `poetry install`
    * Option 2: Use the [complyscribe container image](https://github.com/complytime/complyscribe?tab=readme-ov-file#run-as-a-container)


### 2. Set Permissions for GitHub Actions

The `complyscribe` commands will be run inside of GitHub actions.  These commands often perform `write` level operations against the repo contents.  The GitHub workflows generated in this tutorial make use of [automatic token authentication.](https://docs.github.com/en/actions/security-for-github-actions/security-guides/automatic-token-authentication)  To ensure this is configured correct the following repo settings need to be in place.

*Note: If an alternative method is choosen to provide repo access, such as personal access tokens or GitHub apps, the following steps can be skipped.*

1. Click the `Settings` tab for your GitHub repo 
2. Select `Actions` -> `General` from the left-hand menu
3. Scroll down to `Workflow permissions`
4. Ensure `Read repository contents and packages permissions` is selected
5. Ensure `Allow GitHub Actions to create and approve pull requests` is checked


### 3. Initialize complyscribe Workspace

The `complyscribe init` command will initialize the empty GitHub repository.  Unlike other complyscribe commands, this command is run on the local workstation.  The complyscribe commands can be installed by cloning the [complyscribe](https://github.com/complytime/complyscribe/tree/main) repo and running `poetry install`. Alternatively these commands can be run using the [complyscribe container image](https://github.com/complytime/complyscribe?tab=readme-ov-file#run-as-a-container).

For this tutorial example, we will be authoring a component-definition.

1a. Running complyscribe init using a locally installed complyscribe:

```
complyscribe init --repo-path <path-to-your-repo>
```

1b. Running complyscribe init using a complyscribe container image:

 * *Note: latest image version tag can be found in the [continuouscompliance repo on quay.io](https://quay.io/repository/continuouscompliance/complyscribe?tab=tags).*

```
podman run -v <path-to-your-repo>:/data:rw  complyscribe:<tag> --oscal-model compdef --working-dir /data
```

 * If the local workstation is in SELinux enforcing mode and a permissions error occurs, then the following command should be used instead:
```
podman run -v <path-to-your-repo>:/data:Z  complyscribe:<tag> --oscal-model compdef --working-dir /data
```

 * Once the initiatization runs successfully, the following directories will be created within the local copy of the repository.

```bash
.
├── assessment-plans
├── assessment-results
├── catalogs
├── component-definitions
├── markdown
├── plan-of-action-and-milestones
├── profiles
└── system-security-plans
```

2. Any catalog or profile content needed for the authoring process can now be added.

 * For this example, we will add the NIST SP 800-53 Rev. 5 catalog to our `/catalogs` directory.

```
mkdir catalogs/nist_rev5_800_53
wget https://raw.githubusercontent.com/usnistgov/oscal-content/release-v1.0.5-update/nist.gov/SP800-53/rev5/json/NIST_SP-800-53_rev5_catalog.json -O catalogs/nist_rev5_800_53/catalog.json
```

 * We will also add the NIST SP 800-53 Rev. 5 High Baseline profile to our `profiles/` directory.

```
mkdir profiles/nist_rev5_800_53
wget https://raw.githubusercontent.com/usnistgov/oscal-content/release-v1.0.5-update/nist.gov/SP800-53/rev5/json/NIST_SP-800-53_rev5_HIGH-baseline_profile.json -O profiles/nist_rev5_800_53/profile.json
```

3. Our `profile.json` file contains a reference to our `catalog.json` file.  By default, this path is not resolvable by compliance-trestle, so we need to run the following command to update the `href` value in the JSON.

```
sed -i 's/NIST_SP-800-53_rev5_catalog.json/trestle:\/\/catalogs\/nist_rev5_800_53\/catalog.json/g' profiles/nist_rev5_800_53/profile.json
```

4. Ready-made CI/CD workflows can be copied from the `.github/workflows/` directory within the upstream `trestle-demo` repository into the local trestle workspace. These are the complyscribe actions that will run as changes are made to the repo contents.

 * Copy the required template workflows from the `trestle-demo` repository into the new workspace repository.
```
mkdir -p .github/workflows
wget -O .github/workflows/complyscribe-rules-transform.yml https://raw.githubusercontent.com/RedHatProductSecurity/trestle-demo/refs/heads/main/.github/workflows/complyscribe-rules-transform.yml 
wget -O .github/workflows/complyscribe-create-component-definition.yml https://raw.githubusercontent.com/RedHatProductSecurity/trestle-demo/refs/heads/main/.github/workflows/complyscribe-create-component-definition.yml
```

5. ComplyScribe initial content is now created locally within the new trestle authoring workspace. This content can now be pushed to the remote GitHub repository.
```
git add .
git commit -m "added example NIST SP 800-53 profile and component definition authoring workflow"
git push
```
  *Note: if this is the first git push to the remote GitHub repository, then use `git push -u origin main` rather than `git push`.*


### 4. Create a New Component Definition

Now it's time to run our first complyscribe action within GitHub!  We will go ahead and create our first component definition.

1. Open the new remote GitHub repository in a web browser.
2. Click to the `Actions` tab from the top menu.
3. Click the `ComplyScribe create component definition` action from the left-hand menu.
4. Click `Run Workflow` which will open up a dialog box.
5. Enter the following values:

* _Name of the Trestle profile to use for the component definition:_ `nist_rev5_800_53`
* _Name of the component definition to create:_ `my-first-compdef`
* _Name of the component to create in the generated component definition:_ `test-component`
* _Type of the component (e.g. service, policy, physical, validation, etc.):_ `service`
* _Description of the component to create:_ `Testing complyscribe init`

6. Click `Run Workflow`

Once the workflow job has completed, there will be a new Pull Request containing the files complyscribe generated for the component definition.  After reviewing the committed changes, the Pull Request can then be merged into the main branch!

**Congratulations! We have successfully created a new complyscribe workspace and have an authoring environment!**
