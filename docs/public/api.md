# API Summary (Public)

This page provides a concise reference for common public API operations.

## Authentication

- Use SigV4 signing for API requests. For quick local testing, the server may accept unsigned requests depending on configuration.

## Common operations

- PUT object: `PUT /{bucket}/{key}`
- GET object: `GET /{bucket}/{key}`
- DELETE object: `DELETE /{bucket}/{key}`
- List objects: `GET /{bucket}`

## Example: upload with curl (presigned)

1. Server generates a presigned PUT URL.
2. Upload directly:

```bash
curl -X PUT "${PRESIGNED_URL}" -T file.bin -H "Content-Type: application/octet-stream"
```

## Notes

- For production, always use signed requests and enforce TLS.
