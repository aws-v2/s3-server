# ☁️ MyCloud

**MyCloud** is a portfolio-scale project that reimagines core AWS services with open-source tools.
It’s not meant to replace AWS — but to **learn, explore, and demonstrate distributed systems design** while giving developers free, local *cloud-like* infrastructure.

🧩 **Core Services:** EC2 · S3 · RDS · IAM · Lambda

🛠️ **Stack:** Go · Docker · Redis · NATS · PostgreSQL · Traefik

🔄 **Event Bus:** Built with **NATS**, wrapped in an abstraction layer so all services can publish and subscribe without coupling — forming the “nervous system” of MyCloud.

🎄 **Goal:** Build it all before Christmas.

Just learning how the cloud actually works — one service at a time.

---



### 🚧 Project Status — *Research & Early Build Phase*

| Service                    | Status         | Description                                                                       |
| -------------------------- | -------------- | --------------------------------------------------------------------------------- |
| **🧠 Research & Planning** | ✅ Active       | Reading docs, mapping AWS internals, defining architecture & service interactions |
| **S3**                     | ⚙️ In Progress | Designing file storage layer with MinIO, prefixes, and upload APIs                |
| **IAM**                    | 🧩 Planning    | Tokenless, service-aware authentication and access control system                 |
| **Event Bus**              | 🧩 Planning    | NATS-based event core with an abstraction layer for inter-service messaging       |
| **RDS**                    | 🧩 Planning    | Containerized Postgres provisioning with isolated data instances                  |
| **EC2**                    | 🧩 Planning    | Lightweight compute orchestration using Docker containers                         |
| **Lambda**                 | 🧩 Planning    | Simple serverless-style function runner                                           |




---

> “The best way to understand the cloud is not to build your own small one.” ☁️





Yes — it’s ambitious, but it’s **absolutely possible**. What you’re describing is essentially **a full industrial-grade object storage + distributed persistence engine with integrated semantic search**. Let me break this down carefully.

---

## 1️⃣ Core Vision

You want a **real, production-grade S3-like system** that has:

1. **Durable object storage**:

   * WAL-backed metadata DB
   * Multiple storage sidecars (Disk, NAS, Distributed nodes, MinIO backup) working in **parallel for durability and HA**

2. **High availability & scalability**:

   * Writes replicated to multiple nodes
   * Recovery from crashes via WAL replay
   * Metadata centralized in a fast key-value store (Badger, RocksDB, etc.)

3. **Semantic search**:

   * Each object (or selected objects) is **vectorized** and stored in a **vector database**
   * Allows searching by content, embedding similarity, or metadata queries

---

## 2️⃣ How Semantic Search Fits

### Step-by-step:

1. **Object ingestion**:

   * Client uploads a file (PUT /bucket/object)
   * WAL logs the operation
   * Sidecars save to Disk, NAS, Distributed nodes, and MinIO (parallel writes)

2. **Vectorization**:

   * A **worker (vectorizer)** reads the object content:

     * Text → NLP embeddings (OpenAI embeddings, Sentence Transformers, etc.)
     * Images → vision embeddings
     * Audio → audio embeddings
   * Embeddings are written to a **vector database** (e.g., Milvus, Weaviate, or Pinecone)

3. **Metadata link**:

   * Vector database stores reference to object (bucket + key)
   * Metadata DB still keeps standard info (size, checksum, version)

4. **Querying**:

   * User sends a **semantic query**: e.g., “find all invoices mentioning project X”
   * Query converted to embedding → vector DB returns closest matches
   * API layer resolves object location → returns files

---

## 3️⃣ Architecture Sketch

```
[S3 API Layer]
        |
        v
[WAL Manager]
        |
        +-------------------------------+
        |                               |
[Sidecar Workers]                  [Vectorization Worker]
   |   |   |   |                          |
Disk NAS Dist MinIO                       Vector DB
   |   |   |   |                          |
   +---+---+---+--------------------------+
                  |
             Metadata DB (Badger / RocksDB)
```

* **Parallel writes**: Disk, NAS, Distributed, MinIO backup
* **WAL ensures durability & atomicity**
* **Vector worker asynchronously generates embeddings** for semantic search
* **Metadata DB** keeps object state, versioning, permissions

---

## 4️⃣ Feasibility Notes

1. **Yes, it’s doable**, but it’s a **high-complexity project**.
2. **Embedding all objects** will require CPU/GPU resources depending on content type.
3. **Vector DB integration**:

   * Milvus, Weaviate, Pinecone, Qdrant → can store high-dimensional embeddings
   * Can integrate with your Golang stack via gRPC or REST API
4. **Asynchronous design** is key:

   * Writes → WAL + sidecars → return success
   * Vectorization → async worker → update vector DB
5. **Scale**:

   * Metadata DB should be fast and durable
   * Vector DB can be distributed for large-scale semantic search

---

## 5️⃣ Advantages of This Setup

* **Industrial durability**: WAL + multi-sidecar writes = low risk of data loss
* **High performance**: Parallelism of sidecars + async embedding
* **Extensible**: Can add new sidecars for replication, compression, encryption, or new storage backends
* **Semantic search**: Adds value beyond traditional S3, making objects **intelligently searchable**

---

## 6️⃣ Next Steps

If you want, I can create a **concrete Golang architecture** for this system showing:

* WAL engine
* Parallel sidecar workers for Disk, NAS, Distributed nodes, MinIO backup
* Metadata DB integration
* Vectorization worker for semantic search
* API layer to tie it all together

This would give you a **real industrial-grade blueprint**, not a toy system — exactly what you’re envisioning.

Do you want me to do that next?
