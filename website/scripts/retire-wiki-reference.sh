#!/usr/bin/env bash
set -euo pipefail

repository=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
plan=$repository/_docs/reference-retirement.json
binary=${MW_BIN:-$repository/bin/wm}
profile=
title=
all=false
apply=false

usage() {
    cat <<'HELP'
Usage: bash website/scripts/retire-wiki-reference.sh (--title TITLE | --all) [options]

Preview a reviewed retirement plan, or delete its mapped mediawiki.org reference
pages using wm wiki page delete and each entry's deletionReason.

  --title TITLE     Select exactly one page from the plan
  --all             Select all mapped reference pages (unmapped entries are skipped)
  --plan FILE       Plan JSON (default: _docs/reference-retirement.json)
  --profile NAME    Saved wiki profile; required with --apply
  --apply           Actually delete pages; otherwise only print commands
  --help            Show this help

Build the current CLI with make build before applying. API errors stop the batch.
MW_BIN can select another CLI executable. No redirects or notices are created.
HELP
}

while (($#)); do
    case "$1" in
        --title|--plan|--profile)
            if (($# < 2)) || [[ -z $2 ]]; then echo "Missing value for $1" >&2; exit 1; fi
            case "$1" in --title) title=$2 ;; --plan) plan=$2 ;; --profile) profile=$2 ;; esac
            shift 2 ;;
        --all) all=true; shift ;;
        --apply) apply=true; shift ;;
        --help|-h) usage; exit 0 ;;
        *) echo "Unknown option: $1" >&2; usage >&2; exit 1 ;;
    esac
done
if [[ -n $title && $all == true ]] || [[ -z $title && $all == false ]]; then
    echo 'Choose --title TITLE for one page, or --all for the reviewed batch.' >&2
    exit 1
fi
if [[ $apply == true && -z $profile ]]; then
    echo '--apply requires an explicit --profile with permission to delete pages.' >&2
    exit 1
fi
command -v jq >/dev/null || { echo 'jq is required.' >&2; exit 1; }
if [[ $apply == true ]] && ! command -v "$binary" >/dev/null; then
    echo "CLI executable not found: $binary; run make build first." >&2
    exit 1
fi
# Materialize and validate the selection before running any destructive command.
selection=$(mktemp)
trap 'rm -f -- "$selection"' EXIT
jq -e '(.pages | type == "array") and ([.pages[].title] | length == (unique | length))' "$plan" >/dev/null
jq -c --arg title "$title" --argjson all "$all" '
    .pages[]
    | select($all or .title == $title)
    | select(.title == "Cli/ref" or (.title | startswith("Cli/ref/")))
    | select(.match == "exact" or .match == "alias" or .match == "renamed" or .match == "ancestor")
    | select((.target | type) == "string" and (.deletionReason | type) == "string")
    | select(.target == "https://a-wikimedia-cli.toolforge.org/manual" or
             (.target | startswith("https://a-wikimedia-cli.toolforge.org/manual/")))
    | select((.deletionReason | length) > 0)
    | if (.title | test("[[:cntrl:]]")) or (.deletionReason | test("[[:cntrl:]]"))
      then error("Invalid title or deletion reason") else . end
    | .target as $target
    | if ($target | endswith("/")) or
         ((.deletionReason | contains($target)) | not) or
         (.deletionReason | contains($target + "/"))
      then error("Noncanonical target or deletion reason for " + .title + "; review and regenerate the plan")
      else . end
' "$plan" > "$selection"
if ! test -s "$selection"; then echo 'No mapped reference pages matched; nothing deleted.' >&2; exit 1; fi

count=0
while IFS= read -r entry; do
    page=$(jq -r '.title' <<< "$entry")
    reason=$(jq -r '.deletionReason' <<< "$entry")
    command=("$binary" --no-interaction wiki page delete --wiki https://www.mediawiki.org/w/api.php --title "$page" --reason "$reason")
    if [[ -n $profile ]]; then command+=(--profile "$profile"); fi
    printf '$ '; printf '%q ' "${command[@]}"; printf '\n'
    if [[ $apply == true ]]; then
        if ! "${command[@]}"; then echo "Stopped at: $page" >&2; exit 1; fi
        printf 'Deleted: %s\n' "$page"
    fi
    count=$((count+1))
done < "$selection"
if [[ $apply == true ]]; then
    printf 'Deleted %d reference pages.\n' "$count"
else
    printf 'Preview: %d pages; nothing changed. Add --profile NAME --apply to delete.\n' "$count"
fi
