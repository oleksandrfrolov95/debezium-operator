Debezium Operator
=================

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

A Kubernetes Operator for managing [Debezium](https://debezium.io/) connectors to streamline Change Data Capture (CDC) workflows.

The **Debezium Operator** is a Kubernetes operator that automates the management of Debezium connectors using custom resources. It is written in Go using the Operator SDK and controller‑runtime, and it manages the lifecycle of Debezium connectors by creating, updating, deleting and reconciling connector configurations through a CRD.

Features
--------

*   **CRD-Based Management:** Create a DebeziumConnector custom resource (CR) to specify the desired connector configuration.
    
*   **Automated Reconciliation:** The operator periodically reconciles each CR to ensure that the external Debezium connector configuration matches the CR. If manual changes are made to the connector, they are detected and reverted.
    
*   **Status Updates:** The operator updates the CR's status field ConnectorStatus with the external state (e.g. RUNNING, PAUSED, TASK\_FAILED, or UNKNOWN).

Prerequisites
-------------

*   Kubernetes cluster (v1.20+ recommended)
    
*   Debezium instance with REST API endpoints available (v2.0+ recommended)
    

Installation
------------

### Using Helm

TODO
    
Custom Resource Definition
--------------------------

The operator uses a CRD named DebeziumConnector in the API group api.debezium/v1alpha1. 
For example:

```
apiVersion: api.debezium/v1alpha1
kind: DebeziumConnector
metadata:
  labels:
    app.kubernetes.io/name: debezium-operator
  name: debeziumconnector-sample
spec:
  debeziumHost: debezium.local
  config:
    name: my-connector
    connector.class: io.debezium.connector.mysql.MySqlConnector
    tasks.max: "1"
    database.hostname: mysql
    database.port: "3306"
    database.user: debezium
    database.password: dbz
    database.server.id: "184054"
    database.server.name: my-app-connector
    database.include.list: mydb
    
```

Monitoring
----------

Metrics are exposed on port 8080 by default. Sample Prometheus configuration:

``` 
scrape_configs:
  - job_name: 'debezium-operator'
    metrics_path: /metrics
    static_configs:
      - targets: ['debezium-operator.debezium-operator-ns.svc:8080']

```