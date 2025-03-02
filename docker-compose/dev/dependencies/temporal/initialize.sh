#!/bin/bash

temporal operator namespace update --history-archival-state enabled -n $TEMPORAL_CLI_NAMESPACE
temporal operator namespace update --visibility-archival-state enabled -n $TEMPORAL_CLI_NAMESPACE
