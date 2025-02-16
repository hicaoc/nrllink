#/bin/sh

#bd4two.nrlptt.com
hostlist='nrlptt.com bh4tdv.nrlptt.com ba1gm.nrlptt.com bd4vki.nrlptt.com'

time=`date "+%Y%m%d%H%M%S"`

go build 

for i in $hostlist ; do     
echo "deploying to $i"
   scp udphub root@$i:
   ssh root@$i "cd /nrllink; mv udphub udphub.$time ; cp /root/udphub . ; systemctl restart nrllink"
done
