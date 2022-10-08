#!/bin/bash

CID=$(/opt/keycloak/bin/kcadm.sh create clients --target-realm $2 --set clientId=$1 --set 'redirectUris=["http://*"]' --id)
/opt/keycloak/bin/kcadm.sh create clients/${CID}/client-secret --target-realm $2