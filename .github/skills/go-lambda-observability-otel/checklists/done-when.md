# Done-When Checklist — go-lambda-observability-otel

- [ ] Provider bootstrap is centralized.
- [ ] Stable resource attributes are configured where required.
- [ ] Inbound HTTP or SQS boundaries are instrumented intentionally.
- [ ] Important outbound dependencies are instrumented where operationally relevant.
- [ ] Span names are stable and low-cardinality.
- [ ] Metric labels are bounded and low-cardinality.
- [ ] Correlation context is preserved through changed paths.
- [ ] Backend/exporter details do not leak into domain or application layers.
- [ ] Sensitive payloads and secrets are not recorded in telemetry.
- [ ] Tests cover changed propagation or instrumentation behavior where contractual.
