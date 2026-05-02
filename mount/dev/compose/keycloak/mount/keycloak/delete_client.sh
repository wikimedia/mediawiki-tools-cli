#!/bin/bash

CID=$(/opt/keycloak/bin/kcadm.sh get clients --target-realm $2 --fields id -q clientId=$1 --format csv --noquotes)
/opt/keycloak/bin/kcadm.sh delete clients/${CID} --target-realm $2