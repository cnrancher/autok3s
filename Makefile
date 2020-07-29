SHELL := /bin/bash

# Borrowed from https://stackoverflow.com/questions/18136918/how-to-get-current-relative-directory-of-your-makefile
curr_dir := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

# Borrowed from https://stackoverflow.com/questions/2214575/passing-arguments-to-make-run
rest_args := $(wordlist 2, $(words $(MAKECMDGOALS)), $(MAKECMDGOALS))
$(eval $(rest_args):;@:)

help:
	#
	# Usage:
	#   make autok3s <stage> [only]  :  execute autok3s building process.
	#
	# Stage:
	#   a "stage" consists of serial actions, actions follow as below:
	#     mod -> lint -> build -> containerize -> deploy -> test -> verify -> e2e
	#   for convenience, the name of the "action" also represents the current "stage".
	#   choosing to execute a certain "stage" will execute all actions in the previous sequence.
	#
	# Actions:
	#   -           mod, m  :  download code dependencies.
	#   -          lint, l  :  verify code via `golangci-lint`,
	#                          roll back to `go fmt` and `go vet` if the installation fails.
	#   -         build, b  :  compile code.
	#   -       package, p  :  package docker image.
	#   -        deploy, d  :  push docker image.
	#   -          unit, u  :  run unit tests.
	#   -        verify, v  :  run integration tests.
	#   -           e2e, e  :  run e2e tests.
	#   only executing the corresponding "action" of a "stage" needs the `only` suffix.
	#   integrate with dapper via `BY=dapper`.
	#
	# Example:
	#   -   make autok3s  :  execute `build` stage for autok3s.
	@echo

make_rules := $(shell ls $(curr_dir)/hack/make-rules | sed 's/.sh//g')
$(make_rules):
	@$(curr_dir)/hack/make-rules/$@.sh $(rest_args)

.DEFAULT_GOAL := help
.PHONY: $(make_rules)
