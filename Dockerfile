FROM ubuntu
#FROM scratch
COPY nrllink /nrllink/
COPY start.sh /nrllink/
ENTRYPOINT ["/nrllink/start.sh"]
