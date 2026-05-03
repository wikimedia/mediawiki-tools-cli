#!/bin/bash

CID=$(/opt/keycloak/bin/kcadm.sh get clients --target-realm $2 --fields id -q clientId=$1 --format csv --noquotes)
/opt/keycloak/bin/kcadm.sh get clients/${CID}/client-secret --target-realm $2 --fields value --format csv --noquotes