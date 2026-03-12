Perform a focused security and performance review of the changed files in the philos-video codebase.

Steps:
1. Run `git diff --name-only HEAD` to identify recently changed files.
2. Read each changed Go file carefully.
3. Check each file for the following issues:

**Security:**
- Error details leaked to clients (use generic messages + slog.Error server-side)
- SQL injection (parameterized queries only — no string concatenation in SQL)
- Path traversal (validate video_id / upload_id against expected hex format)
- Hardcoded secrets or credentials
- Missing request body size limits on uploads (`http.MaxBytesReader`)
- Unvalidated X-Forwarded-For (only trust if RemoteAddr is a private IP)
- CORS wildcard (`Access-Control-Allow-Origin: *`) on authenticated endpoints

**Performance:**
- N+1 queries (correlated subqueries in list endpoints — use LEFT JOIN + GROUP BY)
- Missing DB indexes on columns used in WHERE / ORDER BY
- Unbounded goroutines or missing context timeouts
- In-memory state that grows without bounds (maps, slices)

**Correctness:**
- `os.RemoveAll` failures silently ignored (log at WARN level)
- `context.Background()` in long-running goroutines (should use bounded context or shutdown context)
- Race conditions on shared state (should use sync.RWMutex or channels)

**Reliability:**
- Missing graceful shutdown hooks (SIGTERM should drain in-flight work)
- Worker goroutines started without cancellable context

4. Report findings grouped by severity: 🔴 Critical, 🟡 Medium, 🟢 Low.
5. For each finding, include the file path and line number, the problem, and the recommended fix.
6. If no issues are found in a category, explicitly state "None found."
