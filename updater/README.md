# Updater

The service is run in sensor to update the SW/Rules/TI. A configuration file contains all the config required for running of the service.  The service runs infinitely until it is interrupted by a signal.  It completes the task and exits on receiving the signal.  A polling interval in minutes is specified for the service to poll the HC for updates.

* Running
  - go build -o updater main.go
  - ./updater --help
    - List the commandline arguments supported by the service
  - ./updater --config /orion/updater/config/updater-config.json
