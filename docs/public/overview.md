# Overview & Quickstart

This is a lightweight public-facing overview of the s3-compatible server.

## What is this service?

An S3-compatible object storage service used for storing files, supporting presigned URLs and multipart uploads for large objects.

## Key features

- Upload & download objects with S3-like endpoints
- Presigned URL support for temporary, direct uploads/downloads
- Multipart uploads for large objects

## Quickstart (local)

1. Start Postgres and run migrations.
2. Build and run the API server (see developer guide).
3. Use `curl` or the AWS SDK configured for SigV4 to PUT/GET objects.
