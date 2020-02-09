#!/bin/sh
# 
# File:   run_replica.sh
# Author: name-removed
#
# Created on Jun 5, 2019, 6:21:15 PM
#
echo "Making sure no previous replicas are up..."
killall skvbc_replica

echo "Running replica 1..."
../TesterReplica/skvbc_replica -k setB_replica_ -i 0  >& /dev/null &
echo "Running replica 2..."
../TesterReplica/skvbc_replica -k setB_replica_ -i 1 >& /dev/null &
echo "Running replica 3..."
../TesterReplica/skvbc_replica -k setB_replica_ -i 2 >& /dev/null &
echo "Running replica 4..."
../TesterReplica/skvbc_replica -k setB_replica_ -i 3 >& /dev/null &

echo "All UP"