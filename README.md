# nexSign mini (nsm)

`nsm` is a decentralized service written in Go that provides automatic discovery, monitoring, and management for a network of [Anthias](https://www.anthias.io/) digital signage players.

## Project Goals

The primary goal of `nsm` is to create a zero-configuration, resilient, and lightweight monitoring solution for Anthias hosts, especially those running on System-on-Chip (SoC) hardware with limited resources. It achieves this by:

1.  **Automatic Discovery:** `nsm` instances automatically find each other on the local network using mDNS, requiring no central server or manual configuration.
2.  **Distributed State:** It maintains a distributed ledger of all known Anthias hosts using the Tendermint consensus engine. This ledger stores key metadata, ensuring that every node has a consistent view of the network.
3.  **Centralized Access:** It provides a simple web dashboard that aggregates all discovered Anthias players into a single, easy-to-navigate interface, allowing users to quickly switch between the dashboards of different hosts.
4.  **API First:** The service includes a simple REST API to allow for integration with third-party services and automation tools (e.g., n8n, Kestra).

## Architecture

The `nsm` service is composed of several key components:

*   **mDNS Discovery:** A service that constantly browses for and announces `_nsm._tcp` services on the local network.
*   **Tendermint Consensus (ABCI):** A lightweight application that interfaces with Tendermint Core. It manages the state of the distributed ledger, processing transactions to add or update host information.
*   **Anthias Client:** A component responsible for polling the local Anthias instance to gather its status and metadata.
*   **Web Server:** A native Go web server that serves two purposes:
    *   A web dashboard with a persistent top navigation menu and an `<iframe>` to display the selected Anthias host's web UI.
    *   A JSON REST API for external integrations.

## Technology Stack

*   **Language:** Go
*   **Consensus & Networking:** Tendermint Core
*   **Discovery:** mDNS/Zeroconf
*   **Target OS:** Debian Bookworm

## Getting Started

*(This section will be updated with build and run instructions as development progresses.)*