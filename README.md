# Workloader

## Description
Workloader is a tool that helps discover, label, and manage workloads in an Illumio PCE. Functionality includes:

## Installation

### Have Go installed?
Run `go get github.com/brian1917/workloader` or to update `go get -u github.com/brian1917/workloader`. This will download the repo and depenendies and install in your Go path to start using.

### Don't have Go installed?
Download the binary for your operating system from the `bin` folder of this repository. Direct links for each operating systems are here: [Mac](https://github.com/brian1917/workloader/raw/master/bin/workloader-mac), [Linux](https://github.com/brian1917/workloader/raw/master/bin/workloader-linux), and [Windows](https://github.com/brian1917/workloader/raw/master/bin/workloader.exe-windows). No other installation required.


## Usage
`workloader -h`

```
Workloader is a tool that helps discover, label, and manage workloads in an Illumio PCE.

Usage:
  workloader [flags]
  workloader [command]

Available Commands:
  login         Generates a pce.json file for authentication used for all other commands.
  export        Create a CSV export of all workloads in the PCE.
  upload        Create and assign labels from a CSV file. Create and label unmanaged workloads from same CSV.
  traffic       Find and label unmanaged workloads and label existing workloads based on Explorer traffic and an input CSV.
  subnet        Assign environment and location labels based on a workload's network.
  hostparse     Label workloads by parsing hostnames from provided regex functions.
  compatibility Generate a compatibility report for all Idle workloads.
  mode          Change the state of workloads based on a CSV input.
  help          Help about any command

Flags:
  -h, --help   help for workloader

Use "workloader [command] --help" for more information about a command.
```

Each command has its own help menu (e.g., `workloader login -h`)