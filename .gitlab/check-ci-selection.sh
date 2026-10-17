#!/bin/sh
set -eu

# Match the full-pipeline guards in pipelines/commit.yml.
if [ -n "${CI_COMMIT_TAG:-}" ] ||
    [ "${CI_COMMIT_REF_NAME:-}" = "${CI_DEFAULT_BRANCH:-main}" ] ||
    [ "${CI_COMMIT_REF_PROTECTED:-}" = true ] ||
    [ "${CI_MERGE_REQUEST_SOURCE_BRANCH_PROTECTED:-}" = true ] ||
    [ "${CI_COMMIT_MESSAGE_IS_TRUNCATED:-}" = true ]; then
    exit 0
fi
case "${CI_MERGE_REQUEST_EVENT_TYPE:-}" in
    merged_result|merge_train) exit 0 ;;
esac

printf '%s\n' "${CI_COMMIT_MESSAGE:-}" | awk '
    /^CI-(Only|Skip):/ {
        mode = $0
        sub(/:.*/, "", mode)
        modes[mode] = 1
        group = $0
        sub(/^[^:]*:[ \t]*/, "", group)
        sub(/\r$/, "", group)
        sub(/[ \t]+$/, "", group)
        if (group !~ /^(integration|preview-release|docs|tools)$/) {
            print "Invalid " mode " group: " group
            print "Use integration, preview-release, docs or tools; one group per trailer."
            invalid = 1
        }
    }
    END {
        if (modes["CI-Only"] && modes["CI-Skip"]) {
            print "Use either CI-Only or CI-Skip trailers, not both."
            invalid = 1
        }
        exit invalid
    }
'
