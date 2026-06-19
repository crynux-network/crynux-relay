# Configuration Requirements

Default configuration values MUST NOT be set in Go code.

All default values MUST be declared explicitly in the YAML config files for each build or runtime template, including example, test, and e2e configs.

Go code MUST return an error when a required configuration item is missing. It MUST NOT continue with an implicit zero value.
