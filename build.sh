#!/bin/zsh
go clean
go build -ldflags "-X github.com/brian1917/workloader/utils.Version=$(cat version) -X github.com/brian1917/workloader/utils.Commit=$(git rev-list -1 HEAD)"


