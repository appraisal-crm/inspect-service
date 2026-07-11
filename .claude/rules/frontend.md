---
paths:
  - "frontend/**/*.tsx"
  - "frontend/**/*.ts"
---

# Frontend conventions

## Four separate SPAs

| App                         | Users      | Key flows                                         |
|-----------------------------|------------|---------------------------------------------------|
| `frontend/client-portal/`   | Client     | Submit request, track status, download report     |
| `frontend/appraiser/`       | Appraiser  | Manage requests, assign inspector, send report    |
| `frontend/inspector/`       | Inspector  | Mobile-first, view assignments, upload photos     |
| `frontend/admin/`           | Admin      | User management, system monitoring                |

## Rules

- Auth via Keycloak OIDC — use `react-oidc-context` or `keycloak-js`
- Request status is server-authoritative — never derive it on the client
- Photo upload: get a presigned S3 URL from the backend, upload directly to S3 from the browser
- TypeScript strict mode — no `any`
- Generate API types from Swagger schemas — do not write them by hand
- Inspector app must work on mobile (responsive, touch-friendly)
