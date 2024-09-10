FROM busybox:latest

RUN mkdir -p /opt/pinger/etc/pinger.conf.d
ADD ./pinger /opt/pinger
ADD ./pinger.conf /opt/pinger/etc/
ENTRYPOINT ["/opt/pinger/pinger", "-config-file", "/opt/pinger/etc/pinger.conf", "-config-overlays", "/opt/pinger/etc/pinger.conf.d"]
