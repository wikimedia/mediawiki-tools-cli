#!/bin/bash

# Exit if any command fails
set -e
# Exit if any variable is unset
set -u
# Exit if any command in a pipeline fails
set -o pipefail

# Print the last modified time of this file
RUN_LAST_MODIFIED=$(stat -c %y /mwdd/entrypoint-jobrunner.sh)
JOBRUNNER_SITES_FILE=/mwdd-runtime/jobrunner-sites
echo "Last modified: $RUN_LAST_MODIFIED"
echo "Running..."

# Keep running, as long as the file hasn't changed
# This means file changes will exit, and cause a restart
while [ "$RUN_LAST_MODIFIED" = "$(stat -c %y /mwdd/entrypoint-jobrunner.sh)" ]; do
    # If the file doesnt exist, or is empty, sleep and skip
    if [ ! -f "$JOBRUNNER_SITES_FILE" ] || [ ! -s "$JOBRUNNER_SITES_FILE" ]; then
        echo "No sites to run jobs for, sleeping..."
        sleep 1
        continue
    fi

    # Iterate through lines of the jobrunner sites file and run the runJobs.php script for each
    for site in $(cat "$JOBRUNNER_SITES_FILE"); do
        echo "Running jobs for $site"
        php /var/www/html/w/maintenance/runJobs.php --wiki $site
    done

    # Sleep for 1 second at the end of the main loop
    sleep 1
done

echo "File changed, exiting..."