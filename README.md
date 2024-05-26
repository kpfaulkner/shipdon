# Shipdon

Shipdon is a Mastodon client which runs on Windows, MacOS and Linux.
It is currently still in development but is stable enough to be a usable client.

## Install

Pre-built binaries will soon be available at https://github.com/kpfaulkner/shipdon 

## Build

Go 1.21 is required to build Shipdon.

Instructions for building: go build .

This will generate a single binary called `shipdon` in the current directory.

Upon first run, Shipdon will request authentication (via OAuth) to your preferred 
Mastodon instance and will create a configuration file in the 
user's home directory called ~/.shipdon/config.yaml

Currently Shipdon is still in development and is rather "noisy" debug message wise
but will be cleaned up in the near future.


### Notes:
 Some icons are from https://iconscout.com/free-icon-pack/google-material-vol-1
