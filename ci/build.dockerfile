FROM busybox:glibc
 
COPY build/vulcan_linux_amd64 /usr/local/bin/vulcan
ENTRYPOINT ["vulcan"]
