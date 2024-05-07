# docker-compose

This development environment utilizes `docker compose` to orchestrate docker containers that make deliver the environment.

The `docker compose` commands that are run are abstracted away but can be seen where possible by asking for verbose output `-v=2`.

As the development environment is made up of many YAML files this command has been provided to facilitate running `docker compose` commands directly.

All needed `--file` options, as well as the correct `--project-directory` etc will be added to your input.
