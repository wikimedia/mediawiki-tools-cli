#!/bin/bash

USERID=$(/opt/keycloak/bin/kcadm.sh get users --target-realm $2 --fields id -q username=$1 --format csv --noquotes)
/opt/keycloak/bin/kcadm.sh delete users/${USERID} --target-realm $2