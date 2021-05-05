# Workloader

## Description
Workloader is a tool that helps manage resources in an Illumio PCE.

## Documentation
Each command is documented (input formats, flags, etc.) within the help menu. To see the list of commands run `workloader -h`. To see the documentation for a command run something like `workloader wkld-import -h`.

## Installation

### Have Go installed?
Run `go get github.com/brian1917/workloader` or to update `go get -u github.com/brian1917/workloader`. This will download the repo and depenendies and install in your Go path to start using.

### Don't have Go installed?
Download the binary for your operating system from the [releases](https://github.com/brian1917/workloader/releases) section of this repository. No other installation required. Your first command should be to add a PCE so run `workloader pce-add`.
