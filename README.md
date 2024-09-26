prometheus-csv-discovery
------------------------

# Overview
This is a Prometheus http service discovery that reads a CSV file and returns the content as a service discovery file.
It can be used for csv local files or remote files over http(s).

# Configuration
Please see the `config_example.yaml` for configuration example. The configuration file is in YAML format and 
can have multiple entries in the `discovery_targets` section. 
To use the service discovery, the call should be made to the `/prometheus-sd-targets` endpoint with the 
query parameter `discover` set to the value of the `name` attribute, like:
```shell
curl http://localhost:9911/prometheus-sd-targets?discover=abc
```

To set the address for the service discovery, use the `SERVER_ADDR` environment variable, default `:9911    `.

The service can take two arguments:
- `-config` - the path to the configuration file, default `config.yaml`
- `-v` - print the version and exit

# Endpoints
- `/prometheus-sd-targets` - discovery based on configuration file
- `/metrics` - service metrics
	