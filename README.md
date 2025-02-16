# debezium-operator
Golang k8s debezium operator
## Init commands list:
```
- operator-sdk init --domain debezium --plugins go/v4 --repo github.com/oleksandrfrolov95/debezium-operator
- operator-sdk create api --group=api --version=v1alpha1 --kind=DebeziumConnector
```
