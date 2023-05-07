# gravwellPinger
Simple pinging daemon that then sends the results to Gravwell

## Building and Installation
Build using any standard Go toolchain 1.20 or better

A service file is included that expects the binary to be in /opt/pinger

If you want to run the service as anything other than root you will need to provide the raw packet capability:

```
setcap cap_net_raw+ep /opt/pinger/pinger
```


### Installation

A basic build and installation procedure is as follows:
```
CGO_ENABLED=0 go build

sudo mkdir -p /opt/pinger/pinger.conf.d
sudo chown nobody:nogroup /opt/pinger
sudo chown nobody:nogroup /opt/pinger/pinger.conf.d

sudo cp pinger /opt/pinger/pinger
sudo chown nobody:nogroup /opt/pinger/pinger
setcap cap_net_raw+ep /opt/pinger/pinger

sudo cp pinger.conf /opt/pinger/
sudo chown nobody:nogroup /opt/pinger/pinger.conf

sudo cp pinger.service /opt/pinger/
sudo chown root:root /opt/pinger/pinger.service
```

### Service Customization

By default the pinger system will run as the user "nobody" and the group "nogroup" which limits process system access, but it is ofent a good idea to create a dedicated service user with no access to anything but the home directory.

### Kit Installation

A basic Gravwell kit containing a useful dashboard and configuration macro is included.  Use the BUILD instructions in the kit directory to create the unsigned kit and upload it to your Gravwell installation.

A free license for Gravwell is available at [here](https://www.gravwell.io/community-edition)
