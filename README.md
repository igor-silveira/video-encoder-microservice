# Video Encoder Microservice

A Go microservice that encodes MP4 videos into **MPEG-DASH** format for adaptive bitrate streaming. It consumes encoding jobs from RabbitMQ, downloads source videos from Google Cloud Storage, processes them using Bento4, and uploads the encoded output back to GCS.

## Architecture

```
                         ┌──────────────┐
                         │   RabbitMQ   │
                         │  (job queue) │
                         └─────┬────────┘
                               │
                               ▼
┌─────────┐            ┌─────────────────┐           ┌─────────┐
│   GCS   │──download──│  Video Encoder  │──upload───│   GCS   │
│ (input) │            │    Workers      │           │ (output)│
└─────────┘            └────────┬────────┘           └─────────┘
                                │
                     ┌──────────┼──────────┐
                     │          │          │
                     ▼          ▼          ▼
                mp4fragment  mp4dash  PostgreSQL
                (Bento4)    (Bento4)  (job state)
```

### Processing Pipeline

Each video goes through the following stages:

`DOWNLOADING` → `FRAGMENTING` → `ENCODING` → `UPLOADING` → `FINISHING` → `COMPLETED`

1. **Download** — Fetch the source MP4 from the GCS input bucket
2. **Fragment** — Run `mp4fragment` to prepare the file for DASH encoding
3. **Encode** — Run `mp4dash` to generate the MPEG-DASH manifest and segments
4. **Upload** — Concurrently upload all encoded files to the GCS output bucket
5. **Finish** — Clean up local temporary files
6. **Notify** — Publish the result (success or error) to the RabbitMQ notification exchange

If any step fails, the job transitions to `FAILED`, the error is recorded, and the original message is sent to the Dead Letter Exchange.

## Tech Stack

- **Go 1.14** — Application runtime
- **Bento4** (`mp4fragment`, `mp4dash`) — MPEG-DASH video encoding
- **FFmpeg** — Media processing utilities
- **RabbitMQ** — Async job queue and result notifications
- **PostgreSQL** — Job and video state persistence
- **Google Cloud Storage** — Video input/output storage

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)
- A GCP service account with read/write access to Google Cloud Storage

## Getting Started

### 1. Configure environment variables

```bash
cp .env.example .env
```

Edit `.env` with your actual values. See [Configuration](#configuration) for details.

### 2. Set up GCP credentials

Create a service account in GCP with Cloud Storage read/write permissions. Download the JSON credentials file and place it in the project root:

```bash
cp /path/to/your-credentials.json ./bucket-credential.json
```

### 3. Start the services

```bash
docker-compose up -d
```

This starts the app container, PostgreSQL, and RabbitMQ.

### 4. Configure RabbitMQ

Open the RabbitMQ management UI at [http://localhost:15672](http://localhost:15672) (user: `rabbitmq`, pass: `rabbitmq`) and:

1. Create a **fanout exchange** to serve as the Dead Letter Exchange (e.g. `dlx`)
2. Create a **queue** and bind it to that exchange (no routing key needed)
3. Make sure the `RABBITMQ_DLX` value in `.env` matches the exchange name

### 5. Run the encoder

```bash
docker exec <container_name> make server
```

Find the container name with `docker ps`. By default it will be something like `video-encoder_app_1`.

## Configuration

| Variable | Description | Default |
|---|---|---|
| `DB_TYPE` | Database driver | `postgres` |
| `DSN` | Database connection string | — |
| `DB_TYPE_TEST` | Test database driver | `sqlite3` |
| `DSN_TEST` | Test database connection string | `:memory:` |
| `ENV` | Environment name | `dev` |
| `DEBUG` | Enable debug logging | `true` |
| `AUTO_MIGRATE_DB` | Auto-migrate database schema on startup | `true` |
| `LOCAL_STORAGE_PATH` | Temp directory for video processing | `/tmp` |
| `INPUT_BUCKET_NAME` | GCS bucket for source videos | — |
| `OUTPUT_BUCKET_NAME` | GCS bucket for encoded output | — |
| `CONCURRENCY_WORKERS` | Number of parallel job workers | `1` |
| `CONCURRENCY_UPLOAD` | Number of parallel upload goroutines | `50` |
| `RABBITMQ_DEFAULT_USER` | RabbitMQ username | `rabbitmq` |
| `RABBITMQ_DEFAULT_PASS` | RabbitMQ password | `rabbitmq` |
| `RABBITMQ_DEFAULT_HOST` | RabbitMQ host | `rabbit` |
| `RABBITMQ_DEFAULT_PORT` | RabbitMQ port | `5672` |
| `RABBITMQ_DEFAULT_VHOST` | RabbitMQ virtual host | `/` |
| `RABBITMQ_CONSUMER_NAME` | Consumer identifier | `app-name` |
| `RABBITMQ_CONSUMER_QUEUE_NAME` | Queue to consume jobs from | `videos` |
| `RABBITMQ_NOTIFICATION_EX` | Exchange for result notifications | `amq.direct` |
| `RABBITMQ_NOTIFICATION_ROUTING_KEY` | Routing key for notifications | `jobs` |
| `RABBITMQ_DLX` | Dead Letter Exchange name | `dlx` |
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to GCS service account JSON | — |

## Message Format

### Input (publish to the `videos` queue)

```json
{
  "resource_id": "my-resource-id-can-be-a-uuid-type",
  "file_path": "video.mp4"
}
```

- `resource_id` — Your identifier for the video (string)
- `file_path` — Path to the MP4 file inside the input bucket

### Output — Success

Published to the notification exchange on successful encoding:

```json
{
  "id": "bbbdd123-ad05-4dc8-a74c-d63a0a2423d5",
  "output_bucket_path": "my-output-bucket",
  "status": "COMPLETED",
  "video": {
    "encoded_video_folder": "b3f2d41e-2c0a-4830-bd65-68227e97764f",
    "resource_id": "aadc5ff9-0b0d-13ab-4a40-a11b2eaa148c",
    "file_path": "video.mp4"
  },
  "Error": "",
  "created_at": "2020-05-27T19:43:34.850479-04:00",
  "updated_at": "2020-05-27T19:43:38.081754-04:00"
}
```

The `encoded_video_folder` contains the MPEG-DASH manifest and segments in the output bucket.

### Output — Error

Published to the notification exchange when encoding fails:

```json
{
  "message": {
    "resource_id": "aadc5ff9-010d-a3ab-4a40-a11b2eaa148c",
    "file_path": "video.mp4"
  },
  "error": "reason for the error"
}
```

The original message is also routed to the Dead Letter Exchange.

## Running Tests

```bash
make test
```

## Project Structure

```
├── domain/              # Domain entities (Video, Job)
├── application/
│   ├── repositories/    # Database access layer
│   └── services/        # Business logic (encoding pipeline, workers, uploads)
├── framework/
│   ├── cmd/server/      # Application entrypoint
│   ├── database/        # Database connection and migrations
│   ├── queue/           # RabbitMQ integration
│   └── utils/           # JSON validation helpers
├── Dockerfile           # Container image with Bento4 and FFmpeg
├── docker-compose.yaml  # Local dev environment
├── Makefile             # Build and run targets
└── .env.example         # Environment variable template
```
