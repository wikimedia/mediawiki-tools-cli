#!/bin/bash

SCRIPTPATH=$(dirname $(realpath $0))
WORKINGPATH=$(echo $PWD)
cd $SCRIPTPATH

# Force remake of the mwdd files
make internal/mwdd/files/files.go

# Run from source
go run ${SCRIPTPATH}/cmd/cli/main.go $@