FROM scratch
COPY nrllink /usr/bin/nrllink
ENTRYPOINT ["/usr/bin/nrllink"]
