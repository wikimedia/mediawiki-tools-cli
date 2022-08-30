#!/bin/bash

/opt/keycloak/bin/kcadm.sh create users --target-realm $3 --set username=$1 --set enabled=true
/opt/keycloak/bin/kcadm.sh set-password --target-realm $3 --username $1 --new-password $2 --temporary
