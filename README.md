prometheus-csv-discovery
------------------------

# Overview
This is a Prometheus http service discovery that reads a CSV file and returns the content as a service discovery file.
It can be used for csv local files or remote files over http(s).
For local files the file will be re-read if the file is changed using file notify.

# Features 
- Read CSV files on local filesystem or remote files over http(s)
- Support settings to define separator character, exclude header row, define comment character for rows to be ignored
- Support defining the column that should be used for the target name
- Support defining the column and its value that should be used to filter out the targets
- Support defining the column that should be used for the target labels

# Configuration
Please see the `config_example.yaml` for configuration example. The configuration file is in YAML format and 
can have multiple entries in the `discovery_targets` section. 
To use the service discovery, the call should be made to the `/prometheus-sd-targets` endpoint with the 
query parameter `discover` set to the value of the `name` attribute, like:
```shell
curl http://localhost:9911/prometheus-sd-targets?discover=abc
```

To set the address for the service discovery, use the `SERVER_ADDR` environment variable, default `:9911`.

The service can take two arguments:
- `-config` - the path to the configuration file, default `config.yaml`
- `-v` - print the version and exit

# Endpoints
- `/prometheus-sd-targets` - discovery based on configuration file
- `/metrics` - service metrics
	
# Testing 
Copy the `config_example.yaml` to `config.yaml` and update the file with your file path for the `example1`.
Run the discovery.
```shell
curl localhost:9911/prometheus-sd-targets?discover=example1 -s | jq .
```
And the output will be something like:
```json
[
  {
    "targets": [
      "192.168.1.1"
    ],
    "labels": {
      "__meta_active": "true",
      "__meta_description": "New York",
      "__meta_model": "ModelA"
    }
  },
  {
    "targets": [
      "192.168.1.3"
    ],
    "labels": {
      "__meta_active": "true",
      "__meta_description": "Chicago",
      "__meta_model": "ModelC"
    }
  },
  .....
]
```

