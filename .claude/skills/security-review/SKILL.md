---
description: Security review of changed code — finds auth gaps, data leaks, injection risks. Use before merging changes to middleware, auth, file handling, or any public endpoint.
disable-model-invocation: true
argument-hint: <branch, path, or HEAD~1>
---

# Security review: $ARGUMENTS

!`git diff $ARGUMENTS 2>/dev/null | head -600`

## Checklist for this project

### Authentication & authorization
- [ ] JWT validated against Keycloak JWKS — no shared secrets in services
- [ ] Roles extracted from `realm_access.roles` in JWT claims
- [ ] `RequireRoles` middleware present on every protected route
- [ ] No endpoints without auth (except `/health`, `/docs`)
- [ ] Client can only access their own requests (row-level ownership enforced)
- [ ] Inspector can only modify their own assignments

### Input validation
- [ ] All incoming UUIDs validated by format before hitting the DB
- [ ] `playground/validator` used for struct validation
- [ ] Uploaded files: MIME type checked, size limited
- [ ] All SQL uses pgx parameterized queries — no `fmt.Sprintf` in SQL strings

### Data leaks
- [ ] Internal errors (SQL messages, stack traces) not returned to the client
- [ ] Logs do not contain PII (client names, phone numbers, addresses)
- [ ] Presigned S3 URLs issued only to the authorized owner of the request
- [ ] S3 presigned URLs have a short TTL (≤ 15 minutes for uploads)

### Kafka
- [ ] Events contain no sensitive data (passwords, tokens, full PII)
- [ ] Consumers check idempotency before processing

### General
- [ ] Rate limiting enforced at API Gateway level (not in business services)
- [ ] No hardcoded secrets — all credentials come from ENV

## Output format

For each finding:
- **Severity**: Critical / High / Medium / Low
- **Location**: file:line
- **Issue**: what is wrong
- **Fix**: concrete code or pattern to apply
