# Workloader

## Description
Workloader is a tool that helps discover, label, and manage workloads in an Illumio PCE. Functionality includes:
* **csv** - Create labels and unmanaged workloads and label workloads from a CSV.
* **traffic** - Analyze traffic in explorer to identify and label unmanaged workloads and label existing workloads.
* **subnet** - Assign environment and location labels based on a workload's network.
* **hostname** - Assign labels by parsing hostnames using provided regex functions.
* **compatibility** - Get the compatibility status for all workloads in IDLE mode.
* **mode** - Change the mode (idle, build, test, and enforced) of workloads.

## Binaries
Binaries for Mac, Linux, and Windows are located in the `bin` folder of this repository.

## Usage
`workloader -h`

```
Workloader is a tool that helps discover, label, and manage workloads in an Illumio PCE.

Usage:
  workloader [flags]
  workloader [command]

Available Commands:
  login         Generates a pce.json file for authentication used for all other commands.
  csv           Create and assign labels from a CSV file. Create and label unmanaged workloads from same CSV.
  traffic       Find and label unmanaged workloads and label existing workloads based on Explorer traffic and an input CSV.
  subnet        Assign environment and location labels based on a workload's network.
  hostname      Label workloads by parsing hostnames from provided regex functions.
  compatibility Generate a compatibility report for all Idle workloads.
  mode          Change the state of workloads based on a CSV input.
  help          Help about any command

Flags:
  -h, --help   help for workloader

Use "workloader [command] --help" for more information about a command.
```

Each command has its own help menu (e.g., `workloader login -h`)