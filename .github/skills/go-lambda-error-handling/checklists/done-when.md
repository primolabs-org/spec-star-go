# Done When — go-lambda-error-handling

- [ ] Domain/application errors are explicit and transport-agnostic.
- [ ] Wrapped errors preserve cause information with `%w` where context is added.
- [ ] No control flow depends on matching error strings.
- [ ] HTTP expected failures return valid HTTP responses from the adapter.
- [ ] HTTP unexpected failures are safe and diagnosable.
- [ ] SQS per-record failures are handled without accidentally failing the whole batch when partial batch mode is intended.
- [ ] FIFO SQS handling stops after first failure when ordering must be preserved.
- [ ] No panic is used for ordinary business or dependency failures.
- [ ] Tests verify changed error paths and boundary mappings.
