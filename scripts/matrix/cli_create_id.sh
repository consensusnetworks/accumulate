#!/bin/bash
#
# This script uses the accumulate cli to generate an account ID
#
# look for jq and sed
#
j=`which jq`
if [ -z $j ]; then
	echo "jq not found, needed to get account name"
	exit 1
fi
s=`which sed`
if [ -z $s ]; then
	echo "sed not found, needed to get account name"
	exit 1
fi
#
# issue the account generate command to the specified server

if [ -z $1 ]; then
	accid="$($cli account generate -j 2>&1)"
	if [ $? -eq 0 ]; then
           acc=`echo $accid | $j .name | $s 's/\"//g'`
        else
	   echo "cli account generate failed"
	   exit 1
        fi
else
	accid="$($cli account generate -j -s http://$1/v1 2>&1)"
        if [ $? -eq 0 ]; then
	   acc=`echo $accid | $j .name | $s 's/\"//g'`
	else
	   echo "cli account generate failed"
	   exit 1
        fi
fi

# return the generated ID

echo $acc
exit 0

