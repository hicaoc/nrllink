#/bin/sh

#bd4two.nrlptt.com

hostlist='nrlptt.com bh4tdv.nrlptt.com ba1gm.nrlptt.com '

go build 

for i in $hostlist ; do     
echo "deploying to $i"
   scp udphub root@$i:
   ssh root@$i "cd /nrllink; mv udphub udphub.bak ; cp /root/udphub . ; systemctl restart nrllink"
done
