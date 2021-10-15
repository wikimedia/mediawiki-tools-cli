#!/bin/bash
# Modified from https://tarunlalwani.com/post/mysql-master-slave-using-docker/

position_file=/mwdd-connector/mysql_position
file_file=/mwdd-connector/mysql_file

echo "Waiting for mysql replica to start"
/wait-for-it.sh  mysql:3306
/wait-for-it.sh  mysql-replica:3306
# Wait and double check
sleep 1
/wait-for-it.sh  mysql:3306
/wait-for-it.sh  mysql-replica:3306

# TODO add resilience? and wait for the position file to be created...?
# But this is probably okay, as the replica will take a while to start anyway..

# Only save the data if the files don't already exist
# They might have been created during another container startup
if [ ! -e "$position_file" ]; then
    echo "Position file doesnt exist, can't start replication"
    exit 1
fi

echo "* Create replication user"

mysql --host mysql-replica -uroot -p$MYSQL_REPLICA_PASSWORD -AN -e 'STOP SLAVE;';
mysql --host mysql-replica -uroot -p$MYSQL_MAIN_PASSWORD -AN -e 'RESET SLAVE ALL;';

mysql --host mysql -uroot -p$MYSQL_MAIN_PASSWORD -AN -e "CREATE USER '$MYSQL_REPLICATION_USER'@'%';"
mysql --host mysql -uroot -p$MYSQL_MAIN_PASSWORD -AN -e "GRANT REPLICATION SLAVE ON *.* TO '$MYSQL_REPLICATION_USER'@'%' IDENTIFIED BY '$MYSQL_REPLICATION_PASSWORD';"
mysql --host mysql -uroot -p$MYSQL_MAIN_PASSWORD -AN -e 'flush privileges;'


echo "* Set MySQL01 as master on MySQL02"

# Grab the position that should have been set from the first step of mysql-configure when the master was created
MYSQL01_Position=$(<$position_file)
MYSQL01_File=$(<$file_file)

MAIN_IP=$(eval "getent hosts mysql|awk '{print \$1}'")
echo $MAIN_IP
mysql --host mysql-replica -uroot -p$MYSQL_REPLICA_PASSWORD -AN -e "CHANGE MASTER TO master_host='mysql', master_port=3306, \
        master_user='$MYSQL_REPLICATION_USER', master_password='$MYSQL_REPLICATION_PASSWORD', master_log_file='$MYSQL01_File', \
        master_log_pos=$MYSQL01_Position;"

echo "* Set MySQL02 as master on MySQL01"

MYSQL02_Position=$(eval "mysql --host mysql-replica -uroot -p$MYSQL_REPLICA_PASSWORD -e 'show master status \G' | grep Position | sed -n -e 's/^.*: //p'")
MYSQL02_File=$(eval "mysql --host mysql-replica -uroot -p$MYSQL_REPLICA_PASSWORD -e 'show master status \G'     | grep File     | sed -n -e 's/^.*: //p'")

REPLICA_IP=$(eval "getent hosts mysql-replica|awk '{print \$1}'")
echo $REPLICA_IP
mysql --host mysql -uroot -p$MYSQL_MAIN_PASSWORD -AN -e "CHANGE MASTER TO master_host='mysql-replica', master_port=3306, \
        master_user='$MYSQL_REPLICATION_USER', master_password='$MYSQL_REPLICATION_PASSWORD', master_log_file='$MYSQL02_File', \
        master_log_pos=$MYSQL02_Position;"

echo "* Start Replica on both Servers"
mysql --host mysql-replica -uroot -p$MYSQL_REPLICA_PASSWORD -AN -e "start slave;"

echo "Increase the max_connections to 1000"
mysql --host mysql -uroot -p$MYSQL_MAIN_PASSWORD -AN -e 'set GLOBAL max_connections=1000';
mysql --host mysql-replica -uroot -p$MYSQL_REPLICA_PASSWORD -AN -e 'set GLOBAL max_connections=1000';

mysql --host mysql-replica -uroot -p$MYSQL_MAIN_PASSWORD -e "show slave status \G"

echo "MySQL servers created!"
echo "--------------------"
echo
echo Variables available fo you :-
echo
echo MYSQL01_IP       : mysql
echo MYSQL02_IP       : mysql-replica