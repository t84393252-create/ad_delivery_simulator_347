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

## How It Works

### 1. Bid Request Flow

When a user visits a webpage or app with ad space:

```
User visits page → Publisher sends bid request → Ad Delivery Simulator receives request
```

The bid request contains:
- **Impression details**: Ad size, position, format
- **User context**: Device type, geographic location, browser
- **Site information**: Domain, content categories
- **Floor price**: Minimum acceptable bid

### 2. Auction Process

The system runs a real-time auction in <100ms:

```
1. Parse bid request and validate format (OpenRTB 2.5)
2. Fetch active campaigns from database
3. Filter campaigns by:
   - Targeting criteria (geo, device, time of day)
   - Available budget (daily and total)
   - Frequency caps (per user limits)
4. Calculate bid amounts based on:
   - Campaign bid settings (CPM/CPC/CPA)
   - Pacing algorithms (budget distribution)
   - Targeting match quality
5. Run second-price auction:
   - Winner pays second-highest bid + $0.01
   - Ensures fair market pricing
6. Return winning ad creative
```

### 3. Budget Management

Real-time budget tracking prevents overspending:

```
Redis Cache                     PostgreSQL
┌──────────────┐               ┌──────────────┐
│ Daily Budget │←─── Sync ────▶│ Total Budget │
│  (Fast R/W)  │               │ (Persistent) │
└──────────────┘               └──────────────┘
      ↓
  Check & Decrement
      ↓
[Allow/Deny Bid]
```

- **Redis**: Stores hot budget data for microsecond access
- **Atomic operations**: Prevent race conditions
- **Automatic reset**: Daily budgets reset at midnight

### 4. Event Tracking Pipeline

Events flow through an async pipeline for scalability:

```
Event (impression/click) → API endpoint → Validation
                                             ↓
                                     [Buffer in memory]
                                             ↓
                                     Batch processing
                                          ↓     ↓
                                      Kafka   Redis
                                        ↓       ↓
                                  PostgreSQL  Metrics
```

- **Buffering**: Groups events for efficient processing
- **Batch writes**: Reduces database load
- **Kafka streaming**: Enables real-time analytics
- **Metrics aggregation**: Powers dashboards

### 5. Campaign Pacing

Intelligent budget distribution throughout the day:

```python
if spent_today / daily_budget > time_passed_today / 24_hours:
    reduce_bid_rate()  # Slow down spending
else:
    maintain_bid_rate()  # On track
```

This prevents:
- Early budget exhaustion
- Uneven ad delivery
- Missing end-of-day opportunities

### 6. Frequency Capping

Controls ad exposure per user:

```
User sees ad → Increment counter in Redis → Check limits
                        ↓
              [user:campaign:impressions] = 5
                        ↓
              If count > cap → Skip campaign
```

Benefits:
- Prevents ad fatigue
- Improves user experience
- Optimizes reach vs. frequency

### 7. Performance Optimizations

The system achieves 1000+ RPS through:

- **Connection pooling**: Reuses database/Redis connections
- **Goroutines**: Parallel bid processing
- **Caching**: Hot data in Redis
- **Batch processing**: Groups tracking events
- **Async operations**: Non-blocking Kafka writes
- **Circuit breakers**: Prevents cascade failures

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