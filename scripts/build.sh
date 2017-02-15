#!/usr/bin/env bash

ROOTDIR="$( dirname "$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )" )"
BINDIR=$ROOTDIR/bin

set -ex

GOOS=linux go build -o $BINDIR/compile github.com/cloudfoundry/staticfile_builpack

