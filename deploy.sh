#/bin/sh

#hostlist='bd4two.nrlptt.com'

#bh4tdv.nrlptt.com

hostlist='nrlptt.com  ba1gm.nrlptt.com nrlptt.bd4vki.xyz bd4vki.nrlptt.com  ah.nrlptt.com ptt.nrlptt.com bh1osw.nrlptt.com  ham.73ham.com js.nrlptt.com bg1vif.nrlptt.com usa.nrlptt.com nrl.bd4two.site yz.hamuv.com'

#hostlist='ptt.nrlptt.com bh1osw.nrlptt.com'

#hostlist='nrlptt.com bh4tdv.nrlptt.com ba1gm.nrlptt.com bd4vki.nrlptt.com  ah.nrlptt.com   ham.73ham.com '

#hostlist='ham.73ham.com'

#hostlist='nrlptt.com bd4vki.nrlptt.com ah.nrlptt.com'

#hostlist='js.nrlptt.com'

#hostlist='usa.nrlptt.com'

#hostlist='ba1gm.nrlptt.com'

#hostlist='nrlptt.com'

#hostlist='bg1vif.nrlptt.com'

#hostlist='bh4tdv.nrlptt.com'

#hostlist='nrlptt.bd4vki.xyz'
#hostlist='yz.hamuv.com'
#ostlist='bd4vki.nrlptt.com'

time=`date "+%Y%m%d%H%M%S"`

#go build 

for i in $hostlist ; do     
echo "deploying to $i"
   scp udphub root@$i:
   ssh root@$i "cd /nrllink; mv udphub udphub.$time ; cp /root/udphub . ; systemctl restart nrllink"

#ssh root@$i "cd /nrllink; mkdir license"

#scp db/update.sql root@$i:/nrllink/
#ssh root@$i "cd /nrllink; sqlite3 ./udphub.sqlite3 < ./update.sql"

done

