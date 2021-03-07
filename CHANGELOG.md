## v1.7.0 / 2021-03-07

- [ENHANCEMENT] Add serverless deployment
- [ENHANCEMENT] Add default config for custom deployments

## v1.6.0 / 2021-03-05

- [FEATURE] Add cluster alarms
- [FEATURE] Add serverless support via `watcher.RunOnce(...)`
- [FEATURE] Add option to run checks only once via `--run-once` flag
- [ENHANCEMENT] Add insecure option for the kubernetes API server

## v1.5.0 / 2021-03-02

- [ENHANCEMENT] Split CPU and memory resource config for better configuration opportunities

## v1.4.3 / 2021-03-01

- [ENHANCEMENT] Move to native kubernetes incident url

## v1.4.2 / 2021-03-01

- [FIX] Fix binary in docker

## v1.4.1 / 2021-02-28

- [FIX] Fix standard k8s env vars

## v1.4.0 / 2021-02-28

- [ENHANCEMENT] Make dynamic pod and node links for better configuration opportunities
- [ENHANCEMENT] Add alarm type to incident ref
- [FIX] Incident resolution based on alarm type

## v1.3.0 / 2021-02-27

- [ENHANCEMENT] Split pod and node links for better configuration opportunities

## v1.2.4 / 2021-02-27

- [FIX] Fix cron jobs

## v1.2.3 / 2021-02-27

- [FIX] Fix watcher logging

## v1.2.2 / 2021-02-27

- [ENHANCEMENT] Add flags for incident links

## v1.2.1 / 2021-02-27

- [FIX] Fix the incident URLs

## v1.2.0 / 2021-02-27

- [ENHANCEMENT] Add more config options via file, cli or env

## v1.1.0 / 2021-02-25

- [FEATURE] Add node resource limits watcher
- [ENHANCEMENT] Add summary and details to incident reference crd

## v1.0.0 / 2021-02-23

- [FEATURE] Add pod termination and waiting watcher
- [FEATURE] Add pod restarts watcher
- [FEATURE] Add pod resource limits watcher
- [FEATURE] Add node termination watcher
