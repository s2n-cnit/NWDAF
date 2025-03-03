# Plugins

This section contains a list of plugins that are available by the data collector (cmd/dcollector/main.go) to extract
data from multiple type of cores.
There is a common interface that all plugins must implement to be used by the data collector.
Plugins interact with the cores using RPC calls.

Plugins are grouped by the **type** of core they are interacting with. This is important because it does affect the way 
the plugin is configured by env variables.

The plugins are configured by means of ENV variables. Every plugin is using a basic set of ENV variables that are (pkg/configuration/env.go):
- LOG_LEVEL

The plugins are also using a set of ENV variables that are specific to the **core type**. 
These are documented in the README.md of the plugin.
