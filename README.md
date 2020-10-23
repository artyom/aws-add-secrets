Command aws-add-secrets loads secrets from a CSV file to an AWS Secrets
Manager.

CSV file must have a header, which is inspected to find "name", "value", and
an optional "description" columns.

It outputs ARNs of each secret created, or a JSON lines suitable for the
"secrets" section of ECS container task definition if run with an -env flag.
