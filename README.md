# nano-link

A lightweight URL shortener service built with Go and Fiber.

## Features

- Create shortened URLs with optional custom codes
- Track visit statistics for each URL
- API endpoints for URL creation and retrieval
- Automatic cleanup of expired URLs
- Rate limiting to prevent abuse
- Simple web interface

## Quick Start

### Prerequisites

- Go 1.23 or higher
- SQLite (included)

### Installation

1. Clone the repository:
```bash
git clone https://github.com/nijaru/nano-link.git
cd nano-link
```

2. Build the application:
```bash
go build -o nano-link ./cmd/server
```

3. Run the application:
```bash
./nano-link
```

The server starts on port 3000 by default. You can access the web interface at http://localhost:3000.

## Configuration

The application can be configured through environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| PORT | HTTP server port | 3000 |
| DB_PATH | SQLite database path | urls.db |
| BASE_URL | Base URL for shortened links | http://localhost:3000 |
| RATE_LIMIT | Max requests per window | 100 |
| RATE_LIMIT_WINDOW | Rate limit window duration | 1m |
| CLEANUP_INTERVAL | URL cleanup interval | 24h |
| MAX_URL_AGE | Maximum URL lifetime | 720h (30 days) |

You can set these in a `.env` file in the project root.

## API Usage

### Create a Short URL

```
POST /api/shorten
Content-Type: application/json

{
  "url": "https://example.com/very-long-url-that-needs-shortening",
  "custom_code": "example" // Optional
}
```

Response:
```json
{
  "url": {
    "id": 1,
    "original_url": "https://example.com/very-long-url-that-needs-shortening",
    "short_code": "example",
    "visits": 0,
    "created_at": "2023-05-10T15:30:45Z"
  },
  "short_url": "http://localhost:3000/example"
}
```

### Get URL Info

```
GET /api/urls/example
```

Response:
```json
{
  "url": {
    "id": 1,
    "original_url": "https://example.com/very-long-url-that-needs-shortening",
    "short_code": "example",
    "visits": 5,
    "created_at": "2023-05-10T15:30:45Z"
  },
  "short_url": "http://localhost:3000/example"
}
```

### Get Recent URLs

```
GET /api/urls?limit=5
```

Response:
```json
{
  "urls": [
    {
      "url": {
        "id": 1,
        "original_url": "https://example.com/very-long-url-that-needs-shortening",
        "short_code": "example",
        "visits": 5,
        "created_at": "2023-05-10T15:30:45Z"
      },
      "short_url": "http://localhost:3000/example"
    },
    // More URLs...
  ]
}
```

### Get Usage Statistics

```
GET /api/stats
```

Response:
```json
{
  "total_urls": 42,
  "total_visits": 1337,
  "last_created": "2023-05-10T15:30:45Z"
}
```

## Project Structure

- `cmd/server`: Main application entry point
- `internal/`: Internal packages
  - `config`: Application configuration
  - `errors`: Custom error types
  - `handlers`: HTTP handlers
  - `logger`: Custom logging
  - `middleware`: HTTP middleware
  - `models`: Data models
  - `repository`: Data access layer
  - `service`: Business logic
  - `tasks`: Background tasks
- `static/`: Static web assets

## License

[MIT License](LICENSE)
