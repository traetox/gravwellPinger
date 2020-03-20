# gravwellPinger
Simple pinging daemon that then sends the results to Gravwell

## Building and Installation
Build using any standard Go toolchain 1.13 or better

A service file is included that expects the binary to be in /opt/pinger

If you want to run the service as anything other than root you will need to provide the raw packet capability:

```
setcap cap_net_raw+ep /opt/pinger/pinger
```

### Service customization

If you use the included service, be sure to fixup the host list arguments
