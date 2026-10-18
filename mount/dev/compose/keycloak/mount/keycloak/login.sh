#!/bin/bash

/opt/keycloak/bin/kcadm.sh config credentials --server "http://localhost:8080${KC_HTTP_RELATIVE_PATH:-}" --realm master --user "${KEYCLOAK_ADMIN}" --password "${KEYCLOAK_ADMIN_PASSWORD}"
