# Workloader

## Description
Workloader is a tool that helps discover, label, and manage workloads in an Illumio PCE. Functionality includes:

## Installation

### Have Go installed?
Run `go get github.com/brian1917/workloader` or to update `go get -u github.com/brian1917/workloader`. This will download the repo and depenendies and install in your Go path to start using.

### Don't have Go installed?
Download the binary for your operating system from the `bin` folder of this repository. Direct links for each operating systems are here: [Mac](https://github.com/brian1917/workloader/raw/master/bin/workloader-mac), [Linux](https://github.com/brian1917/workloader/raw/master/bin/workloader-linux), and [Windows](https://github.com/brian1917/workloader/raw/master/bin/workloader-win.exe). No other installation required.


## Usage
`workloader -h`

```
Workloader is a tool that helps discover, label, and manage workloads in an Illumio PCE.

Usage:
  workloader [flags]
  workloader [command]

Available Commands:
  login         Verifies existing login or generates a pce.json file for authentication used for all other commands.
  export        Create a CSV export of all workloads in the PCE.
  import        Create and assign labels to a workload from a CSV file. Use the --umwl flag to create and label unmanaged workloads from the same CSV.
  traffic       Find and label unmanaged workloads and label existing workloads based on Explorer traffic and an input CSV.
  subnet        Assign environment and location labels based on a workload's network.
  hostparse     Label workloads by parsing hostnames from provided regex functions.
  mislabel      Display workloads that have no intra App-Group communications to identify potentially mislabled workloads.
  compatibility Generate a compatibility report for all Idle workloads.
  mode          Change the state of workloads based on a CSV input.
  upgrade       Upgrade the VEN installed on workloads by labels or an input hostname list.
  flowupload    Upload flows from CSV file to the PCE.
  flowsummary   Summarize flows by port and protocol between app groups.
  dupecheck     Identifies duplicate hostnames and IP addresses in the PCE.
  help          Help about any command

Flags:
      --debug        Enable debug level logging for troubleshooting.
      --no-prompt    Remove the user prompt when used with update-pce.
      --out string   Output format. 3 options: csv, stdout, both (default "both")
      --update-pce   Command will update the PCE after a single user prompt. Default will just log potentialy changes to workloads.
  -h, --help         help for workloader

Use "workloader [command] --help" for more information about a command.
```

Each command has its own help menu (e.g., `workloader login -h`)