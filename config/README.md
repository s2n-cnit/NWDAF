# Configuration files
Configuration file has the scope for only the configuration of the NWDAF application.
The configuration of the microservices composing the NWDAF application is done through the environment variables.
This allows to have multiple instances of the same microservice with different configurations, managed by the same NWDAF application.
We can then monitor multiple 5G slices with different collectors.

We have chosen YAML because is generally preferred for configuration files due to its readability and support for comments, which make it easier to manage and understand.