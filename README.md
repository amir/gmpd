# gMPD

**gMPD** is a *work in progress* MPD-esque Google Play Music (All Access) Client.

## Requirements
 * [gstreamer](http://gstreamer.freedesktop.org/)
 * A Google Play Music All Access account, with a registered phone.

## How to use
```bash
go get github.com/amir/gmpd
gmpd --email user@gmail.com --password password
```

## Known Issues
 * `idle` is not supported
 * Everything else is half supported, and mostly broken
 * Only tested with [gmpc](http://gmpclient.org)
