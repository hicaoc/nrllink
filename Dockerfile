FROM ubuntu
#FROM scratch
COPY nrllink /nrllink/
RUN mkdir -p /nrllink/conf
RUN mkdir -p /nrllink/data
COPY start.sh /nrllink/
ENTRYPOINT ["/nrllink/start.sh"]