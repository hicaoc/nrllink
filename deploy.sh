#/bin/sh

#hostlist='bd4two.nrlptt.com'
hostlist='nrlptt.com bh4tdv.nrlptt.com ba1gm.nrlptt.com bd4vki.nrlptt.com  ah.nrlptt.com www.bh1osw.com bh1osw.nrlptt.com yz.hamoa.cn ham.73ham.com '

#hostlist='www.bh1osw.com bh1osw.nrlptt.com'
#hostlist='bh1osw.nrlptt.com'

#hostlist='nrlptt.com bh4tdv.nrlptt.com ba1gm.nrlptt.com bd4vki.nrlptt.com  ah.nrlptt.com  yz.hamoa.cn ham.73ham.com '

#hostlist='ham.73ham.com'

#hostlist='nrlptt.com bd4vki.nrlptt.com ah.nrlptt.com'

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

scp -p 27949 udphub root@js.nrlptt.com:
ssh -p 27949 root@js.nrlptt.com "cd /nrllink; mv udphub udphub.$time ; cp /root/udphub . ; systemctl restart nrllink"
