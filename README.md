# Workloader

## Description
Workloader is an open-source CLI tool that leverages the Illumio API to  manage resources and automate common tasks. Workloader's  functionality includes
- Import and export commands that allow management of policy objects via CSV
- Automated labeling commands to assign labels to workloads based on cloud or VM tags, workload subnet, and hostname patterns
- Workload management commands to review comaptibility reports in bulk, manage workload enforcement states via CSV, unpair stale/orphan workloads, and more.
- Reporting commands to view rule usage, traffic exports, and more.

## Getting started
Run `workloader pce-add` to initiate the prompts for the necessary information to connect to the PCE. Subsequent commands use the `pce.yaml` file generated from `pce-add` to authenticate. To see options for authenticating, see the `pce-add` help menu by running `workloader pce-add -h` or `workloader pce-add --help`

## Requirements
The only requirements for workloader are the ability to run the executable and connect to the PCE over HTTPS. The server or workstation running workloader must be able to connect to the PCE like a user would with the browser.

## Permissions
Workloader will be limited to functionality based on the roles assigned to the authenticated user or service account.

## Logging
Logs of all commands and output are stored in a `workloader.log` file.

## Leveraging Workloader in Automation
When a command modifies resources in the PCE, workloader does not trigger the action unless the `--update-pce` flag is included. Without this flag, workloader only simulates the command and logs what would happen. The `--update-pce` flag triggers a prompt for user input to run the command and make the updates. To auto-accept this prompt, as would be needed in automation (i.e., commands running on a cron job), use the `--no-prompt` flag.

## Documentation
Each command is documented within the help menu. The documentation for each command includes instructions, optional flags, and examples (when relevant). To see the list of commands run `workloader -h`. To see the documentation for a command run the command name with `-h` or `--help` such as `workloader wkld-import -h`.

## Installation
No installation required. Download the binary for your operating system from the [releases](https://github.com/brian1917/workloader/releases) section of this repository.

## Feedback or bugs
File issues or feedback requests in the [issues](https://github.com/brian1917/workloader/issues) section of this repository.
