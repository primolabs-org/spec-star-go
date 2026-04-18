# Done-when checklist

- [ ] Logger bootstrap is centralized.
- [ ] Production Lambda paths emit structured JSON logs.
- [ ] Common fields such as `service` and `operation` are stable.
- [ ] HTTP adapters attach request-oriented correlation fields where available.
- [ ] SQS adapters attach message-oriented correlation fields where available.
- [ ] Terminal failures are logged at meaningful boundaries without duplicate noise.
- [ ] Secrets and full sensitive payloads are not logged.
- [ ] Temporary debug logs added for implementation work have been removed.
- [ ] Tests cover changed logging helpers or adapter enrichment where that behavior is contractual.
