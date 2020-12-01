# Contributing

When contributing to this repository, please first discuss the change you wish to make via issue,
email, or any other method with the owners of this repository before making a change. 

Please note we have a [code of conduct](CODE_OF_CONDUCT.md), please follow it in all your interactions with the project.

## Pull Request Process

1. Before submitting a pull request, ensure that the build succeds, tests pass (`make test-acceptance`) and golangci-lint (`make lint`) does not report any issues.
2. If new provisioner attributes have been added, please document them in the README.md file.
3. Do not increase the version in `Makefile`. Version updates are handled during releases.
4. If any of the dependencies have been updated, please state the reason for that change.
