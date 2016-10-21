FROM busybox:glibc
 
COPY ./vulcan_linux_amd64 /usr/local/bin/vulcan
ENTRYPOINT ["vulcan"]
