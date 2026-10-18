#!/bin/bash
# Split from https://tarunlalwani.com/post/mysql-master-slave-using-docker/
# This file grabs the position that we will want to start replication at and stores it in a file.
# This data is then used by the replica to start replicating

position_file=/mwdd-connector/mysql_position
file_file=/mwdd-connector/mysql_file

# Only save the data if both files already contain a valid position. They might
# have been created during another container startup, but an empty file means
# that MySQL was listening before it had finished initializing.
if [ -s "$position_file" ] && [ -s "$file_file" ]; then
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
    MYSQL01_Status=$(mysql --host mysql -uroot -p"$MYSQL_MAIN_PASSWORD" --batch --skip-column-names -e 'SHOW MASTER STATUS' 2>/dev/null || true)
    if [ -n "$MYSQL01_Status" ]; then
        read -r MYSQL01_File MYSQL01_Position _ <<< "$MYSQL01_Status"
        if [ -n "$MYSQL01_File" ] && [[ "$MYSQL01_Position" =~ ^[0-9]+$ ]]; then
            break
        fi
    fi
    position_wait_attempt=$((position_wait_attempt + 1))
    sleep 1
done

if [ -z "$MYSQL01_File" ] || ! [[ "$MYSQL01_Position" =~ ^[0-9]+$ ]]; then
    echo "MySQL did not return a binlog file and position within 60 seconds"
    exit 1
fi

echo "* Saving data to files"

printf '%s\n' "$MYSQL01_Position" > "$position_file"
printf '%s\n' "$MYSQL01_File" > "$file_file"
