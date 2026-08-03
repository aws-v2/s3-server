## API Architecture

Summary of API design and conventions used by the s3 server.

### Principles

- S3-compatible semantics where practical (PUT/GET/HEAD/DELETE, presign, multipart)
- Clear error codes and consistent JSON error bodies for non-S3 flows
- Idempotent operations for retries where applicable

### Key endpoints

- `PUT /{bucket}/{key}`: store object
- `GET /{bucket}/{key}`: retrieve object
- `DELETE /{bucket}/{key}`: delete object
- `POST /{bucket}?uploads`: initiate multipart upload
- `PUT /{bucket}/{key}?partNumber=&uploadId=`: upload part
- `POST /{bucket}/{key}?uploadId=`: complete multipart
- `GET /{bucket}`: list objects

### Authentication

- SigV4 signing for production compatibility. Support for short-lived tokens (JWT) for internal tooling.

### Presigned URLs

- Presigned GET/PUT flow stores limited metadata in `presign_metadata` table and issues time-limited signed URL.

### Error handling

- Return S3-compatible XML errors when requested by clients using AWS headers; otherwise return JSON with `{ code, message, requestId }`.

### Rate limiting & throttling

- API gateway or middleware should enforce request quotas per user or API key. Exponential backoff recommended for clients.
