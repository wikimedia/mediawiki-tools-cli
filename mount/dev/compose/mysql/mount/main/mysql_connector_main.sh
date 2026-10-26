#!/bin/bash
# Split from https://tarunlalwani.com/post/mysql-master-slave-using-docker/
# This file grabs the position that we will want to start replication at and stores it in a file.
# This data is then used by the replica to start replicating

position_file=/mwdd-connector/mysql_position
file_file=/mwdd-connector/mysql_file

# Reuse saved coordinates only when their contents are valid. The old script
# could leave newline-only files when MySQL was still initializing.
MYSQL01_Position=$(cat "$position_file" 2>/dev/null || true)
MYSQL01_File=$(cat "$file_file" 2>/dev/null || true)
if [[ "$MYSQL01_File" =~ ^[^[:space:]]+$ && "$MYSQL01_Position" =~ ^[0-9]+$ ]]; then
    echo "Position file already exists"
    exit 0
fi

rm -f "$position_file" "$file_file"

echo "Waiting for mysql to start"
/wait-for-it.sh  mysql:3306
# Wait and double check
sleep 1
/wait-for-it.sh  mysql:3306

echo "* Wait for the binlog file and position"
position_wait_attempt=0
MYSQL01_File=""
MYSQL01_Position=""
while [ "$position_wait_attempt" -lt 60 ]; do
    MYSQL01_Status=$(mariadb --host mysql -uroot -p"$MYSQL_MAIN_PASSWORD" --batch --skip-column-names -e 'SHOW MASTER STATUS' 2>/dev/null || true)
    if [ -n "$MYSQL01_Status" ]; then
        read -r MYSQL01_File MYSQL01_Position _ <<< "$MYSQL01_Status"
        if [[ "$MYSQL01_File" =~ ^[^[:space:]]+$ && "$MYSQL01_Position" =~ ^[0-9]+$ ]]; then
            break
        fi
    fi
    position_wait_attempt=$((position_wait_attempt + 1))
    sleep 1
done

if ! [[ "$MYSQL01_File" =~ ^[^[:space:]]+$ && "$MYSQL01_Position" =~ ^[0-9]+$ ]]; then
    echo "MySQL did not return a binlog file and position within 60 seconds"
    exit 1
fi

echo "* Saving data to files"

printf '%s\n' "$MYSQL01_Position" > "$position_file"
printf '%s\n' "$MYSQL01_File" > "$file_file"
