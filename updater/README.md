# Updater

The service is run in sensor to update the SW/Rules/TI. A configuration file contains all the config required running of the service.  The service runs infinitely until it is interrupted by a signal.  It completes the task and exits on receiving the signal.

Running
 - go run main.go --help
   - List the commandline arguments supported by the service
