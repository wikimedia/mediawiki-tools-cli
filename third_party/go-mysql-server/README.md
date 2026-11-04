This package is copied from [go-mysql v1.16.0](https://github.com/go-mysql-org/go-mysql/tree/v1.16.0/server) (MIT).
The upstream client and protocol packages remain dependencies.

The decoder retains prepared-statement parameter types across executions,
including new-params-bound flag 0, and binds streamed string/blob parameters
without requiring inline bytes. Streamed state is limited to 16 MiB per statement
and cleared after execution, reset or close, including failed executions.
Remove this copy when the upstream decoder supports both behaviors.
