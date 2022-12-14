FROM ubuntu
#FROM scratch
COPY nrllink /nrllink/
ENTRYPOINT ["/nrllink/udphub"]
