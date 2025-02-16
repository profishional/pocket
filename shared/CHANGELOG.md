# Changelog

All notable changes to this module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.0.0.19] - 2023-02-02

- Add `KeyPair` interface
- Add logic to create new keypairs, encrypt/armour them and decrypt/unarmour them

## [0.0.0.18] - 2023-01-31

- Match naming conventions in `Param` protobuf file

## [0.0.0.17] - 2023-01-27

- Add `Param` and `Flag` protobufs for use in updating merkle tree

## [0.0.0.16] - 2023-01-24

- Add `ConsensusPacemaker` interface that is implemented by the consensus module

## [0.0.0.15] - 2023-01-20

- Remove `address []byte` argument from `InsertPool` function in `PostgresRWContext`

## [0.0.0.14] - 2023-01-19

- Rewrite `interface{}` to `any`

## [0.0.0.13] - 2023-01-18

- Create `block.proto` which consolidates the definition of a `Block` protobuf under `shared/core/types`

## [0.0.0.12] - 2023-01-11

- Deprecated `GetBlocksPerSession()` and `GetServiceNodesPerSessionAt()` in favour of the more general parameter getter function `GetParameter()`

## [0.0.0.11] - 2023-01-11

- Make the events channel hold pointers rather than copies of the message

## [0.0.0.10] - 2023-01-10

- Updated modules constructor to accept a `bus` and not a `runtimeMgr` anymore
- Registering modules with the `bus` via `RegisterModule` method

## [0.0.0.9] - 2023-01-04

- Removed `ValidatorMap() ValidatorMap` from `ConsensusModule` interface
- Added `GetIsClientOnly()` to `P2PConfig`

## [0.0.0.8] - 2023-01-03

- Added `PoolNames.FriendlyName` method
- Renamed enums as per code-review
- Updated `InitParams` logic to use genesisState instead of hardcoded values

## [0.0.0.7] - 2022-12-21

- Updated to use the new centralized config and genesis handling
- Created `Actor` struct under `coreTypes`
- Created `Account` struct under `coreTypes`
- Created `PoolNames` enum under `coreTypes`
- Updated module to use the new `coreTypes`
- Simplified `*Module` interfaces
- Updated tests and mocks

## [0.0.0.6] - 2022-12-14

- Added `GetMaxMempoolCount`

## [0.0.0.5] - 2022-12-06

- Change the `bus` to be a pointer receiver rather than a value receiver in all the functions it implements

## [0.0.0.4] - 2022-11-30

Debug:

- `ResetToGenesis` - Added the ability to reset the state to genesis
- `ClearState` - Added the ability to clear the state completely (height 0 without genesis data)

Configs:

- Updated the test generator to produce deterministic keys
- Added `trees_store_dir` to persistence configs
- Updated `LocalNet` configs to have an empty `tx_indexer_path` and `trees_store_dir`

## [0.0.0.3] - 2022-11-14

### [#353](https://github.com/pokt-network/pocket/pull/353) Remove topic from messaging

- Removed topic from messaging
- Added `PocketEnvelope` as a general purpose wrapper for messages/events
- Added utility methods to `Pack` and `Unpack` messages
- Replaced the switch cases, interfaces accordingly

## [0.0.0.2] - 2022-10-12

### [#235](https://github.com/pokt-network/pocket/pull/235) Config and genesis handling

- Updated to use `RuntimeMgr`, available via `GetRuntimeMgr()`
- Segregate interfaces (eg: `GenesisDependentModule`, `P2PAddressableModule`, etc)
- Updated tests and mocks

## [0.0.0.1] - 2022-09-30

- Used proper `TODO/INVESTIGATE/DISCUSS` convention across package
- Moved TxIndexer Package to Utility to properly encapsulate
- Add unit test for `SharedCodec()`
- Added `TestProtoStructure` for testing
- Flaky tests troubleshooting - https://github.com/pokt-network/pocket/issues/192
- More context here as well: https://github.com/pokt-network/pocket/pull/198

### [#198](https://github.com/pokt-network/pocket/pull/198) Flaky tests

- Time mocking abilities via https://github.com/benbjohnson/clock and simple utility wrappers
- Race conditions and concurrency fixes via sync.Mutex

## [0.0.0.0] - 2022-08-25

### [#163](https://github.com/pokt-network/pocket/issues/163) Minimization

- Moved all shared structures out of the shared module
- Moved structure responsibility of config and genesis to the respective modules
- Shared interfaces and general 'base' configuration located here
- Moved make client code to 'debug' to clarify that the event distribution is for the temporary local net
- Left multiple `TODO` for remaining code in test_artifacts to think on removal of shared testing code

<!-- GITHUB_WIKI: changelog/shared -->
