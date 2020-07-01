# Workloader

## Description
Workloader is a tool that helps discover, label, and manage workloads in an Illumio PCE. Functionality includes:

## Installation

### Have Go installed?
Run `go get github.com/brian1917/workloader` or to update `go get -u github.com/brian1917/workloader`. This will download the repo and depenendies and install in your Go path to start using.

### Don't have Go installed?
Download the binary for your operating system from the [releases](https://github.com/brian1917/workloader/releases) section of this repository. No other installation required.

## Usage
`workloader -h`

```
Workloader is a tool that helps discover, label, and manage workloads in an Illumio PCE.

  Usage:
	workloader [command]

  PCE Management Commands:
	pce-add              Adds a PCE to the pce.yaml file.
	pce-remove           remove a pce from pce.yaml file
	pce-list             List all PCEs in pce.yaml.
	get-default          Get the default PCE to be used for all commands targeting a single PCE (i.e., do not transfer data between PCEs).
	set-default          Changes the default PCE to be used for all commands targeting a single PCE (i.e., do not transfer data between PCEs).

  Import/Export Commands:
	wkld-export          Create a CSV export of all workloads in the PCE.
	wkld-import          Create and assign labels to existing workloads and/or create unmanaged workloads (using --umwl) from a CSV file.
	ipl-export           Create a CSV export of all IP lists in the PCE.
	ipl-import           Create and update IP Lists from a CSV.
	flow-import          Upload flows from CSV file to the PCE.
	label-export         Create a CSV export of all labels in the PCE.
	label-rename         Rename labels based on CSV input.
	template-import      Import an Illumio segmentation template.
	  
  Automated Labeling Commands:
	traffic              Find and label unmanaged workloads and label existing workloads based on Explorer traffic and an input CSV.
	subnet               Assign environment and location labels based on a workload's network.
	hostparse            Label workloads by parsing hostnames from provided regex functions.
	snow-sync            Label existing workloads and (optionally) create unmanaged workloads from data stored in ServiceNow CMDB.

  Workload Management Commands:
	compatibility        Generate a compatibility report for all Idle workloads.
	mode                 Change the state of workloads based on a CSV input.
	upgrade              Upgrade the VEN installed on workloads by labels or an input hostname list.
	get-pk               Get a pairing key.
	unpair               Unpair workloads through an input file or by a combination of labels and hours since last heartbeat.
	delete               Delete unmanaged workloads by HREFs provided in file.

  Label Management Commands:
	labels-delete-unused Delete labels that are not used.

  Reporting Commands:
	mislabel             Display workloads that have no intra App-Group communications to identify potentially mislabled workloads.
	dupecheck            Identifies duplicate hostnames and IP addresses in the PCE.
	flowsummary          Summarize flows from explorer. Two subcommands: appgroup (available now) and env (coming soon).
	explorer             Export explorer traffic data enhanced with some additional information (e.g., subnet, default gateway, interface name, etc.).
	nic                  Export all network interfaces for all managed and unmanaged workloads.

  PCE-to-PCE Commands:
	wkld-to-ipl          Create IP lists based on workloads labels.

  Version Command:
	version              Print workloader version.
  
Use "workloader [command] --help" for more information on a command.
```

Each command has its own help menu (e.g., `workloader pce-add -h`)