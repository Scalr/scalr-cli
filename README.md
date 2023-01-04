# Scalr CLI
[![e2e workflow](https://github.com/Scalr/scalr-cli/actions/workflows/testing.yml/badge.svg)](https://github.com/Scalr/scalr-cli/actions/workflows/testing.yml)

"scalr" is a command-line tool that communicates directly with the Scalr API.

## Beta Software
Please note that this is currently beta software and might not work as expected at all times. No guarantees are given. Use at your own risk.
If you find something that is broken, please run it again with the "-verbose" flag and post the output as an [issue](https://github.com/Scalr/scalr-cli/issues).

## Installation

Installing the Scalr cli tool is very straightforward as it's distributed as a single static binary. Simply download the zipped binary from the [releases](https://github.com/Scalr/scalr-cli/releases) section that corresponds to your architecture and (preferably) unpack it somewhere in your PATH.

## Basic usage
```
user@server ~$ scalr

Usage: scalr [OPTION] COMMAND [FLAGS]

  'scalr' is a command-line interface tool that communicates directly with the Scalr API

Examples:
  $ scalr -help
  $ scalr -help get-workspaces
  $ scalr get-foo-bar -flag=value
  $ scalr -verbose create-foo-bar -flag=value -flag2=value2
  $ scalr create-foo-bar < json-blob.txt

Environment variables:
  SCALR_HOSTNAME  Scalr Hostname, i.e example.scalr.io
  SCALR_TOKEN     Scalr API Token
  SCALR_ACCOUNT   Default Scalr Account ID, i.e acc-tq8cgt2hu6hpfuj  

Options:
  -version       Shows current version of this binary
  -help          Shows documentation for all (or specified) command(s)
  -verbose       Shows complete request and response communication data
  -configure     Run configuration wizard
  -update        Updates this tool to the latest version by downloading and replacing current binary
  -autocomplete  Enable shell tab auto-complete
```

## Configure:
Before the CLI can be used, you will need to configure what Scalr URL and [Token](https://docs.scalr.com/en/latest/api/index.html#scalr-api) to use.
This can be done using environment variables (SCALR_HOSTNAME and SCALR_TOKEN) or a configuration file.
Run the CLI with the -configure flag to run the configuration wizard.

An optional environment variable called SCALR_ACCOUNT can be used to set a default account ID. When running a command that has either an -account or -account-id flag
(and not manually specified with flags), it will automatically be set to that default account ID.

```
user@server ~$ scalr -configure
Scalr Hostname [ex: example.scalr.io]: example.scalr.io
Scalr Token (not echoed!): 
Default Scalr Account-ID [ex: acc-tq8cgt2hu6hpfuj]: acc-tq8cgt2hu6hpfuj
Configuration saved in /home/user/.scalr/scalr.conf
```

## List available flags for a specific command:
```
user@server ~$ scalr -help create-environment

Usage: scalr create-environment [FLAGS] [< json-blob.txt]

  Create a new environment in the account.

  Environment's are collections of related workspaces that correspond to functional areas, SDLC stages,
  projects or any grouping that is required.

  An account can have multiple environments.

  Workspaces within an environment are where Terraform configurations are run to deploy infrastructure,
  and where state files are stored.

  An Environment can have set of policy groups assigned that are applied to all workspaces in the environment.
  The Environment can also have variables, credentials, registry modules, and VCS providers
  that are available to every workspace.

Flags:
  -account-id=STRING                          The account that owns this environment. [*required]
  -cloud-credentials-id=STRING
  -cost-estimation-enabled=BOOLEAN            Indicates if the cost estimation should be performed for `runs` in the environment.
  -default-provider-configurations-id=STRING  Provider configurations used in the environment workspaces by default.
  -name=STRING                                The name of the environment. [*required]
  -policy-groups-id=STRING
```

For commands that CREATES or UPDATES something, you can chose to set the values using flags (-flag=value) OR use a raw JSON blob
as you would do when communicating directly with the Scalr API.

## Example using flags:
```
user@server ~$ scalr create-environment -name=development -account-id=acc-t2fcrq6h1v3nf0g
```

## Examples using JSON blob:
```
user@server ~$ scalr create-environment < json-blob.txt

user@server ~$ echo '
> {
>      "data": {
>          "attributes": {
>              "name": "development"
>          },
>          "relationships": {
>              "account": {
>                  "data": {
>                      "id": "acc-t2fcrq6h1v3nf0g",
>                      "type": "accounts"
>                  }
>              }
>          },
>          "type": "environments"
>      }
> } 
> ' | scalr create-environment
```

## List required flags:

The -help output will tell you which flags are required. However, it can be hard to find them if the flag list is long.
Simply running the command without flags will tell you which required flags have missing values.

```
user@server ~$ scalr lock-workspace
Missing required flag(s): [workspace]
```

## Update CLI version:
```
user@server ~$ scalr -update
Latest version is 0.9.0, which is different from current installed version 0.8.0. 
Downloading version 0.9.0...
Replacing current binary with downloaded version... 
All done! Your binary is now at version 0.9.0 
```

## Shell tab auto-complete
Enabling tab auto-completion will make working with the CLI more efficient, as the shell will automatically show you available commands, flags and options.
Simply activate it with "scalr -autocomplete". You will have to restart your shell before using it. Remember to press TAB twice to make the shell show you the available options.

```
user@server ~$ scalr list-e
list-endpoints           list-environment-tags    list-environments        list-event-definitions

user@server ~$ scalr list-environments -
-filter-account=          -filter-environment=      -filter-latest-run-date=  -filter-tag=              -include=                 -query=                   -sort=

user@server ~$ scalr list-environments -sort=
account                  cost-estimation-enabled  created-at               created-by-email         name

user@server ~$ scalr list-environments -sort=account -
-filter-account=          -filter-environment=      -filter-latest-run-date=  -filter-tag=              -include=                 -query=                   -sort=

user@server ~$ scalr list-environments -sort=account -include=
account                          created-by                       policy-groups                    tags
cloud-credentials                default-provider-configurations  provider-configurations

user@server ~$ scalr list-environments -sort=account -include=tags,
tags,account                          tags,created-by                       tags,policy-groups                    tags,tags
tags,cloud-credentials                tags,default-provider-configurations  tags,provider-configurations

user@server ~$ scalr list-environments -sort=account -include=tags,created-by
```

## List available commands:
```
user@server ~$ scalr -help

Access Policy:
  create-access-policy  Create an Access Policy
  delete-access-policy  Delete Access Policy
  get-access-policies   List Access Policies
  get-access-policy     Get an Access Policy
  update-access-policy  Update an Access Policy

Access Token:
  create-access-token            Create an Access Token
  create-agent-pool-token        Create an Agent Pool Access Token
  delete-access-token            Delete an Access Token
  get-access-token               Get an Access Token
  list-agent-pool-access-tokens  List Agent Pool Access Tokens
  update-access-token            Update an Access Token

Account:
  get-account     Get an Account
  update-account  Update Account

Account Blob Settings:
  delete-account-blob-settings   Delete Blob Settings
  get-account-blob-settings      Get Blob Settings
  replace-account-blob-settings  Replace Blob Settings
  update-account-blob-settings   Update Blob Settings

Agent:
  delete-agent  Delete an Agent
  get-agent     Get an Agent
  get-agents    List Agents

Agent Pool:
  create-agent-pool  Create an Agent Pool
  delete-agent-pool  Delete an Agent Pool
  get-agent-pool     Get an Agent Pool
  get-agent-pools    List Agent Pools
  update-agent-pool  Update an Agent Pool

Apply:
  get-apply      Get an Apply
  get-apply-log  Apply Log

Configuration Version:
  create-configuration-version    Create a Configuration Version
  download-configuration-version  Download Configuration Version
  get-configuration-version       Get a Configuration Version
  get-configuration-versions      List Configuration Versions

Cost Estimate:
  get-cost-estimate            Get a Cost Estimate
  get-cost-estimate-breakdown  Cost breakdown JSON output
  get-cost-estimate-log        Cost Estimate log

Endpoint:
  create-endpoint  Create an Endpoint
  delete-endpoint  Delete an Endpoint
  get-endpoint     Get an Endpoint
  list-endpoints   List Endpoints
  update-endpoint  Update Endpoint

Environment:
  create-environment  Create an Environment
  delete-environment  Delete an Environment
  get-environment     Get an Environment
  list-environments   List Environments
  update-environment  Update Environment

Event Definition:
  list-event-definitions  List Event Definitions

Module:
  create-module          Publish a Module
  delete-module          Unpublish a Module
  get-module             Get a Module
  list-modules           List Modules
  resync-module          Resync a Module
  resync-module-version  Resync a Module Version

Module Version:
  get-module-version    Get a Module Version
  list-module-versions  List Module Versions

Permission:
  get-permission   Get a Permission
  get-permissions  List Permissions

Ping:
  ping  Ping

Plan:
  get-json-output            JSON Output
  get-plan                   Get a Plan
  get-plan-log               Plan Log
  get-sanitized-json-output  Sanitized JSON Output

Policy:
  get-policy  Get a Policy

Policy Check:
  get-policy-check       Get a Policy Check
  get-policy-checks-log  Policy Check Log
  list-policy-checks     List Policy Checks
  override-policy        Override Policy

Policy Group:
  create-policy-group               Create a Policy Group
  create-policy-group-environments  Create policy group environments relationships
  delete-policy-group               Delete a Policy Group
  delete-policy-group-environments  Delete policy group's environment relationship
  get-policy-group                  Get a Policy Group
  list-policy-groups                List Policy Groups
  update-policy-group               Update a Policy Group
  update-policy-group-environments  Update policy group environments relationships

Provider Configuration:
  create-provider-configuration  Create a Provider configuration
  delete-provider-configuration  Delete a Provider configuration
  get-provider-configuration     Get a Provider configuration
  list-provider-configurations   List Provider configurations
  update-provider-configuration  Update a Provider configuration

Provider Configuration Link:
  create-provider-configuration-link            Attach a Provider configuration to the workspace
  delete-provider-configuration-workspace-link  Delete a Provider configuration workspace link
  get-provider-configuration-link               Get a Provider configuration link
  list-provider-configuration-links             List Provider configuration workspace links
  update-provider-configuration-link            Update a Provider configuration link

Provider Configuration Parameter:
  create-provider-configuration-parameter  Create a Provider configuration parameter
  delete-provider-configuration-parameter  Delete a Provider configuration parameter
  get-provider-configuration-parameter     Get a Provider configuration parameter
  list-provider-configuration-parameters   List Provider configuration parameters for specific provider configurations
  update-provider-configuration-parameter  Update a Provider configuration parameter

Role:
  create-role  Create a Role
  delete-role  Delete a Role
  get-role     Get a Role
  get-roles    List Roles
  update-role  Update a Role

Run:
  cancel-run             Cancel a Run
  confirm-run            Apply a Run
  create-run             Create a Run
  discard-run            Discard a Run
  download-policy-input  Download a Policy Input
  get-run                Get a Run
  get-runs               List Runs
  get-runs-queue         List Runs Queue

Run Trigger:
  create-run-trigger  Create a Run Trigger.
  delete-run-trigger  Delete a Run Trigger
  get-run-trigger     Get a Run Trigger

Service Account:
  create-service-account  Create a Service Account
  delete-service-account  Delete a Service Account
  get-service-account     Get a Service Account
  get-service-accounts    List Service Accounts
  update-service-account  Update a Service Account

State Version:
  get-current-state-version   Get Workspace's Current State Version
  get-state-version           Get a State Version
  get-state-version-download  Download State Version
  list-state-versions         List Workspace's State Versions

Tag:
  create-tag  Create a Tag
  delete-tag  Delete a Tag
  get-tag     Get a Tag
  list-tags   List Tags
  update-tag  Update a Tag

Team:
  create-team  Create a Team
  delete-team  Delete a Team
  get-team     Get a Team
  get-teams    List Teams
  update-team  Update a Team

Usage Statistic:
  list-usage-statistics  List Scalr Usage Statistics

User:
  create-user               Create a User
  delete-user               Delete a User
  get-account-users         List Account to User relationships
  get-user                  Get a User
  get-users                 List Users
  invite-user-to-account    Invite a User to the Account
  remove-user-from-account  Remove a User from the Account
  update-user               Update a User

Variable:
  create-variable  Create a Variable
  delete-variable  Delete a Variable
  get-variable     Get a Variable
  get-variables    List Variables
  update-variable  Update a Variable

Vcs Provider:
  create-vcs-provider  Create a VCS Provider
  delete-vcs-provider  Delete a VCS Provider
  get-vcs-provider     Get a VCS Provider
  list-vcs-providers   List VCS Providers
  update-vcs-provider  Update a VCS Provider

Webhook:
  create-webhook  Create Webhook
  delete-webhook  Delete a Webhook
  get-webhook     Get a Webhook
  list-webhooks   List Webhooks
  update-webhook  Update Webhook

Workspace:
  add-remote-state-consumers      Add remote state consumers
  add-workspace-tags              Add tags to the workspace
  create-workspace                Create a Workspace
  delete-remote-state-consumers   Delete remote state consumers
  delete-workspace                Delete a Workspace
  delete-workspace-tags           Delete workspace's tags
  get-workspace                   Get a Workspace
  get-workspaces                  List Workspaces
  list-remote-state-consumers     List remote state consumers
  list-workspace-tags             List workspace's tags
  lock-workspace                  Lock a Workspace
  replace-remote-state-consumers  Replace remote state consumers
  replace-workspace-tags          Replace workspace's tags
  resync-workspace                Resync a Workspace
  set-schedule                    Set scheduled runs for the workspace
  unlock-workspace                Unlock a Workspace
  update-workspace                Update a Workspace
```
