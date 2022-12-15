#!/bin/bash

workdir="/nrllink"
conf="$workdir/conf/udphub.yaml"
dbfile="$workdir/data/udphub.sqlite3"

if [ ! -f "$conf" ] ; then
        cp $workdir/udphub.yaml $workdir/conf/
fi


if [  ! -f "$conf" ] ; then
        cp $workdir/udphub.sqlite3 $workdir/data/
fi


$workdir/udphub -c $workdir/conf/udphub.yaml
