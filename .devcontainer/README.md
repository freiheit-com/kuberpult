# Standalone Devcontainer

A standalone devcontainer including quick setup for Claude, Revolution MCP, GitHub (via github personal access tokens) 
and the fdc standard tech stack. This setup is not the devcontainer setup used in the fdc-standard-setup itself, but
instead meant as a reference for projects that do not have their own devcontainer setup yet or for members that need an ad-hoc devcontainer for PoCs and want to avoid running Claude on the host.

## Devcontainer stack / assumptions
Some assumptions about the project stack were made when this setup was created.
Most likely, some adjustments will be required before you can use this setup.

### Claude
This setup assumes that you are using Claude Code as the AI harness.
The devcontainer is setup in such a way that you have to sign in once from within the container (just like you would from the host). After that, the sign-in information is persisted in a volume and you stay signed in through container rebuilds.

If you use a different agent / harness, you will have to adjust the docker file and replace the post-start action in `scripts/post-start-claude`.

### GitHub with PAT (personal access token)
The best way to authenticate from a devcontainer to GitHub is via GitHub Apps with properly scoped permissions.
Since we don't have access to GitHub Apps with the fdc GitHub account, we use GitHub PATs in this template.
If you can use GitHub Apps instead, you have to modify the github related setup code in `scripts/post-start-github`.

Avoid using GitHub sign-in methods that are not scoped to specific repositories and permissions when working with agents.

### Revolution
This setup assumes that you use the revolution MCP. The revolution MCP should work out of the box if you set the required environment variables and secrets.

## Setup

1. Install Docker and either the [devcontainer CLI](https://github.com/devcontainers/cli)
   or VS Code's *Dev Containers* extension.
2. Copy this directory into your project's workspace directory as `.devcontainer/`
3. Replace `fdc-standard-setup` with a project-specific name in `./devcontainer/devcontainer.json`.
4. Non-secret, project-wide values (e.g. the Revolution workspace/product IDs) live in
   [project.env](project.env). Add a GitHub token as `DEVCONTAINER_GH_TOKEN`, plus any MCP credentials, to your
   project secrets file. Update `PROJECT_SECRETS_FILE` in [project.env](project.env). 
   An example for the secrets file content is:
   - SECRET.env.local
      ```
      REVOLUTION_USER_ID=...
      REVOLUTION_MCP_TOKEN=...
      DEVCONTAINER_GH_TOKEN=...
      ```
5. Run the following commands to build and run the container
   ```
   .devcontainer/devcontainer.sh up --workspace-folder .
   .devcontainer/devcontainer.sh exec --workspace-folder . zsh
   ```
   On the first execution, `claude` will require configuration and sign-in.

   Using VS Code's *Dev Containers* extension instead? Launch VS Code with
   `.devcontainer/code.sh .` rather than opening it normally, then reopen the folder in
   a container -- the extension only sees the secrets from step 2 if VS Code itself was
   started with them present.

## Known limitations
This setup has some known limitations that will be addressed in the future. 
If you find limitations while working with this setup, please open a PR that documents them
briefly in this section of the readme.

### Agent access to secrets
Since the secrets are available in the container as environment variables, a running 
AI agent has access to them. 
Keep this in mind when deciding which secrets should be exposed within the container.
Ideally, only expose secrets with minimal blast radius and short lifespans.

Status: The SRE team is actively working on a solution for this.

### Missing network restrictions
This setup does not include network restrictions like egress filtering.

Status: The SRE team is working on a solution for this.

### Rootless docker
This setup currently does not work with rootless docker.

Status: This is planned, but the SRE team is currently not working on this. Support is welcome.

### Plaintext secrets on the host
The secret injection into the container uses a secrets file that is stored unencrypted on the host system.
It is done like this because each project at fdc implements different methods to work with secrets locally and
we cannot cover them all here. We strongly suggest that you replace the plaintext secrets on the host with 
your projects default method for secret handling on the host.
You can e.g. fetch secrets on-demand from a cloud secret store (preferred) like GCP Secret manager or Azure Key Vault. 
Alternatively, you can keep encrypted secrets on the host and decrypt them on demand when the container is started, e.g.
using SOPS.

Status: This is a limitation of keeping this template as generic as possible. There is currently no plan to address this.
