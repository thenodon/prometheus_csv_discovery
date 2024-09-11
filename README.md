prometheus-csv-discovery
------------------------

# Overview
This is a Prometheus http service discovery that reads a CSV file and returns the content as a service discovery file.
It can be used for csv local files or remote files over http(s).

# Configuration
Please see the `config_example.yaml` for configuration example.

To set the address for the service discovery, use the `SERVER_ADDRESS` environment variable, default `:8080`.

The service can take two arguments:
- `-config` - the path to the configuration file, default `config.yaml`
- `-v` - print the version and exit

