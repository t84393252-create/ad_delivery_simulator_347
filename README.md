# Ad Delivery Simulator

A comprehensive ad delivery microservice demonstrating real-time bidding (RTB), campaign management, and performance tracking. Built with Go, Redis, Kafka, and PostgreSQL.

## Problem Context

### The Digital Advertising Challenge

Modern digital advertising operates at massive scale with billions of ad requests processed daily across the internet. Publishers need to monetize their content, advertisers want to reach their target audience efficiently, and users expect relevant, non-intrusive ads. This creates several technical challenges:

1. **Real-Time Decision Making**: Ad auctions must complete in under 100 milliseconds to avoid impacting page load times
2. **Budget Management**: Campaigns need precise budget control to prevent overspending while maximizing reach
3. **Targeting Precision**: Ads must reach the right audience based on multiple criteria (geography, device, behavior)
4. **Scale and Performance**: Systems must handle thousands of requests per second with high availability
5. **Fraud Prevention**: Invalid traffic and click fraud can waste advertising budgets
6. **Privacy Compliance**: Modern regulations require careful handling of user data

### What This Service Solves

This Ad Delivery Simulator provides a production-ready solution for the core components of an advertising platform:

#### For Publishers
- **Maximized Revenue**: Second-price auctions ensure fair market value for ad inventory
- **Fill Rate Optimization**: Multiple advertisers compete for each impression
- **Quality Control**: Frequency capping prevents user fatigue from repetitive ads

#### For Advertisers  
- **Budget Control**: Real-time budget tracking prevents overspending
- **Campaign Pacing**: Algorithms distribute budget evenly throughout the day
- **Precise Targeting**: Reach specific audiences based on geography, device type, and user segments
- **Performance Tracking**: Real-time metrics for impressions, clicks, and conversions

#### For Engineers
- **Scalable Architecture**: Event-driven design with Kafka handles growth
- **Low Latency**: Redis caching and optimized auction logic ensure sub-100ms responses
- **High Throughput**: Batch processing and async operations handle 1000+ RPS
- **Observable System**: Prometheus metrics and structured logging for monitoring

### Real-World Applications

This system architecture is used by:
- **Ad Exchanges**: Connecting publishers and advertisers in real-time
- **Demand-Side Platforms (DSPs)**: Managing advertiser campaigns programmatically  
- **Supply-Side Platforms (SSPs)**: Optimizing publisher inventory yield
- **Ad Networks**: Aggregating and selling publisher inventory
- **Marketing Platforms**: Running performance marketing campaigns

## Features

- **Real-Time Bidding Engine**: OpenRTB 2.5 compliant bid request/response system
- **Campaign Management**: Complete CRUD operations with budget control and pacing
- **Second-Price Auction**: Efficient auction mechanism with targeting and frequency capping  
- **Event Tracking**: Real-time impression, click, and conversion tracking
- **Performance Metrics**: Prometheus metrics and Grafana dashboards
- **Event Streaming**: Kafka-based event processing for scalability
- **Caching Layer**: Redis for real-time operations and budget management

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│  REST API   │────▶│   Gin HTTP  │
└─────────────┘     └─────────────┘     └─────────────┘
                            │
                    ┌───────┴────────┐
                    │                │
            ┌───────▼────┐   ┌──────▼──────┐
            │  Auction   │   │  Campaign   │
            │   Engine   │   │  Service    │
            └───────┬────┘   └──────┬──────┘
                    │                │
            ┌───────▼────────────────▼──────┐
            │       Tracking Service        │
            └───────────┬───────────────────┘
                        │
        ┌───────────────┼───────────────────┐
        │               │                   │
   ┌────▼────┐    ┌────▼────┐        ┌────▼────┐
   │  Redis  │    │  Kafka  │        │Postgres │
   └─────────┘    └─────────┘        └─────────┘
```

## Quick Start

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- Make

### Installation

1. Clone the repository:
```bash
git clone https://github.com/t84393252-create/ad_delivery_simulator_347.git
cd ad_delivery_simulator_347
```

2. Start infrastructure services:
```bash
make docker-up
```

3. Run the application:
```bash
make run
```

The API will be available at `http://localhost:8080`

## API Endpoints

### Bidding

#### POST /api/v1/bid-request
Process a real-time bid request.

```bash
curl -X POST http://localhost:8080/api/v1/bid-request \
  -H "Content-Type: application/json" \
  -d '{
    "id": "bid-123",
    "imp": [{
      "id": "imp-1",
      "banner": {
        "w": 300,
        "h": 250
      },
      "bidfloor": 0.5
    }],
    "device": {
      "ua": "Mozilla/5.0",
      "ip": "192.168.1.1",
      "devicetype": 1,
      "geo": {
        "country": "US"
      }
    },
    "user": {
      "id": "user-456"
    }
  }'
```

### Campaign Management

#### POST /api/v1/campaigns
Create a new campaign.

```bash
curl -X POST http://localhost:8080/api/v1/campaigns \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Summer Sale Campaign",
    "advertiser_id": "adv-123",
    "budget_daily": 1000.00,
    "budget_total": 10000.00,
    "bid_type": "CPM",
    "bid_amount": 2.50,
    "targeting_rules": {
      "geo_targeting": ["US", "CA"],
      "device_types": ["1", "2"]
    },
    "frequency_capping": {
      "impression_cap": 10,
      "time_window": "24h"
    },
    "start_date": "2024-01-01T00:00:00Z"
  }'
```

#### GET /api/v1/campaigns/{id}
Get campaign details.

```bash
curl http://localhost:8080/api/v1/campaigns/{campaign-id}
```

#### GET /api/v1/campaigns/{id}/performance
Get campaign performance metrics.

```bash
curl http://localhost:8080/api/v1/campaigns/{campaign-id}/performance?date=2024-01-15
```

### Tracking

#### POST /api/v1/track/impression
Track an ad impression.

```bash
curl -X POST http://localhost:8080/api/v1/track/impression \
  -H "Content-Type: application/json" \
  -d '{
    "campaign_id": "campaign-uuid",
    "user_id": "user-123",
    "session_id": "session-456"
  }'
```

#### POST /api/v1/track/click
Track an ad click.

```bash
curl -X POST http://localhost:8080/api/v1/track/click \
  -H "Content-Type: application/json" \
  -d '{
    "campaign_id": "campaign-uuid",
    "user_id": "user-123",
    "session_id": "session-456"
  }'
```

## Configuration

Configuration can be set via `config/config.yaml` or environment variables:

```yaml
server:
  port: 8080
  mode: "development"

database:
  host: "localhost"
  port: 5432
  user: "aduser"
  password: "adpass123"
  database: "addelivery"

redis:
  host: "localhost"
  port: 6379

kafka:
  brokers:
    - "localhost:9092"
```

Environment variables use the prefix `AD_DELIVERY_` (e.g., `AD_DELIVERY_SERVER_PORT=8080`).

## Development

### Running Tests

```bash
# Unit tests
make test

# Benchmarks
make bench
```

### Code Formatting

```bash
make fmt
make lint
```

### Database Migrations

The application automatically runs migrations on startup. Manual migration:

```bash
make migrate
```

## Load Testing

Run the included load testing script:

```bash
make load-test
```

This simulates 1000 concurrent bid requests per second.

## Monitoring

- **Metrics**: Prometheus metrics available at `/metrics`
- **Health Check**: `/health`
- **Grafana**: http://localhost:3000 (admin/admin)

## Performance

The system is designed to handle:
- 1000+ bid requests per second
- Sub-100ms response times
- Real-time metric updates
- Horizontal scaling via Kafka consumers

## Project Structure

```
.
├── cmd/server/         # Application entry point
├── internal/           # Business logic
│   ├── auction/        # Bidding engine
│   ├── campaign/       # Campaign management
│   ├── tracking/       # Event tracking
│   └── models/         # Data models
├── pkg/                # Reusable packages
│   ├── redis/          # Redis client
│   └── kafka/          # Kafka client
├── api/                # HTTP handlers
├── config/             # Configuration
└── tests/              # Test files
```

## Technologies

- **Go 1.21**: Core application
- **Gin**: HTTP framework
- **PostgreSQL**: Persistent storage
- **Redis**: Caching and real-time operations
- **Kafka**: Event streaming
- **Prometheus**: Metrics collection
- **Docker**: Containerization

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License.

## Support

For issues and questions, please open an issue on GitHub.