# Plugin shared resources

This directory contains shared resources for the plugin like declared environment variables, common functions, etc.

## Environment variables
Each plugin type can have different environment variables based on the **Core Type** it is interacting with.
The ENV variables are declared in the file 5G_core_env.go such that their names are unique and do not conflict with other plugins,
but most importantly they have a standardized name that is not hardcoded in the plugin code.