#/bin/sh

#hostlist='bd4two.nrlptt.com'

#bh4tdv.nrlptt.com

hostlist='182.92.158.141 m.nrlptt.com  ba1gm.nrlptt.com  nrlptt.bd4vki.xyz bd4vki.nrlptt.com  www.bg1vif.com ah.nrlptt.com ptt.nrlptt.com bh1osw.nrlptt.com  ham.73ham.com js.nrlptt.com  usa.nrlptt.com nrl.bd4two.site yz.hamuv.com'

#hostlist='bh1osw.nrlptt.com'

#hostlist='182.92.158.141'

#hostlist='nrlptt.com bh4tdv.nrlptt.com ba1gm.nrlptt.com bd4vki.nrlptt.com  ah.nrlptt.com   ham.73ham.com '

#hostlist='ham.73ham.com'

#hostlist='nrlptt.com bd4vki.nrlptt.com ah.nrlptt.com'

#hostlist='js.nrlptt.com ah.nrlptt.com nrl.bd4two.site yz.hamuv.com'

#hostlist='usa.nrlptt.com'

#hostlist='nrl.bd4two.site js.nrlptt.com ham.73ham.com'

#hostlist='ba1gm.nrlptt.com'

#hostlist='js.nrlptt.com'

#hostlist='ah.nrlptt.com'

#hostlist='www.bg1vif.com'

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

scp udphub root@192.168.35.40:nrllink/nrllink/

