# System Architecture & Flow Diagrams

## 📊 Complete System Overview

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                                   EXTERNAL CLIENTS                                   │
├─────────────────────────────────────────────────────────────────────────────────────┤
│  [Publishers]          [Advertisers]           [Analytics]          [Monitoring]     │
│      ↓                      ↓                       ↓                    ↓           │
│  Bid Requests         Campaign Mgmt           Get Metrics         Health Checks      │
└──────┬──────────────────────┬────────────────────┬───────────────────┬──────────────┘
       │                      │                    │                   │
       ▼                      ▼                    ▼                   ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                            LOAD BALANCER / API GATEWAY                              │
│                                  (HTTP/HTTPS)                                       │
└──────────────────────────────────┬──────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              GIN HTTP ROUTER                                        │
│  ┌────────────┬──────────────┬──────────────┬──────────────┬──────────────┐       │
│  │ /bid-request│  /campaigns  │    /track    │   /metrics   │   /health    │       │
│  └─────┬──────┴──────┬───────┴──────┬───────┴──────┬───────┴──────┬───────┘       │
└────────┼─────────────┼──────────────┼──────────────┼──────────────┼────────────────┘
         │             │              │              │              │
         ▼             ▼              ▼              ▼              ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                            BUSINESS LOGIC LAYER                                     │
├─────────────────────────────────────────────────────────────────────────────────────┤
│ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐               │
│ │   AUCTION    │ │   CAMPAIGN   │ │   TRACKING   │ │  ANALYTICS   │               │
│ │   ENGINE     │ │   SERVICE    │ │   SERVICE    │ │   SERVICE    │               │
│ ├──────────────┤ ├──────────────┤ ├──────────────┤ ├──────────────┤               │
│ │ • Bid eval   │ │ • CRUD ops   │ │ • Events     │ │ • Metrics    │               │
│ │ • Targeting  │ │ • Budget     │ │ • Batching   │ │ • Reports    │               │
│ │ • Scoring    │ │ • Pacing     │ │ • Validation │ │ • Aggregation│               │
│ │ • Winner     │ │ • Frequency  │ │ • Streaming  │ │ • Real-time  │               │
│ └──────┬───────┘ └──────┬───────┘ └──────┬───────┘ └──────┬───────┘               │
└────────┼────────────────┼────────────────┼────────────────┼────────────────────────┘
         │                │                │                │
         ▼                ▼                ▼                ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              DATA ACCESS LAYER                                      │
├─────────────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐                    │
│  │     REDIS       │  │     KAFKA       │  │   POSTGRESQL    │                    │
│  │   (Cache)       │  │   (Streams)     │  │   (Database)    │                    │
│  ├─────────────────┤  ├─────────────────┤  ├─────────────────┤                    │
│  │ • Budgets      │  │ • Bid events    │  │ • Campaigns     │                    │
│  │ • Freq caps    │  │ • Impressions   │  │ • Creatives     │                    │
│  │ • Hot data     │  │ • Clicks        │  │ • History       │                    │
│  │ • Counters     │  │ • Conversions   │  │ • Reports       │                    │
│  │ • Rate limits  │  │ • Analytics     │  │ • Transactions  │                    │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘                    │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

## 🔄 Complete Bid Request Flow

```
[1] Publisher Website
         │
         ▼
    Ad Space Available
         │
         ▼
┌─────────────────┐
│  Bid Request    │──────────────────────────────────┐
└─────────────────┘                                  │
         │                                           ▼
         ▼                                   [Request Details]
    Gin Router                               • User ID
         │                                   • Site info
         ▼                                   • Ad size
  Auction Engine                             • Floor price
         │
         ├──────[2] Fetch Campaigns──────▶ Campaign Service
         │                                        │
         │                                        ▼
         │                                  Redis (Budget Check)
         │                                        │
         │◀──────── Active Campaigns ─────────────┘
         │
         ├──────[3] Check Targeting───────────────┐
         │                                        │
         │   • Geography match?                   │
         │   • Device type match?                 │
         │   • User segment match?                │
         │   • Time window match?                 │
         │                                        │
         ├──────[4] Frequency Check──────▶ Redis │
         │                                   │    │
         │◀────── Under cap? ────────────────┘    │
         │                                        │
         ├──────[5] Calculate Bids───────────────┘
         │
         │   For each eligible campaign:
         │   • Base bid amount
         │   • Performance multiplier
         │   • Budget pacing factor
         │   • Competition estimate
         │
         ├──────[6] Run Auction──────────────────┐
         │                                        │
         │   Sort bids by price                  │
         │   Winner = highest bidder              │
         │   Price = 2nd highest + $0.01         │
         │                                        │
         ├──────[7] Decrement Budget─────▶ Redis │
         │                                        │
         ├──────[8] Log Event────────────▶ Kafka │
         │                                        │
         ▼                                        ▼
    [Response]                              [Async Processing]
    • Ad creative                           • Analytics
    • Tracking URLs                         • Reporting
    • Win notification                      • Billing
```

## 📈 Event Tracking Flow

```
User Sees/Clicks Ad
         │
         ▼
┌─────────────────┐
│ Tracking Event  │
│ (Browser/App)   │
└────────┬────────┘
         │
         ▼
    HTTP POST
    /track/{type}
         │
         ▼
    Gin Router
         │
         ▼
  Tracking Service
         │
    ┌────┴────┐
    │Validate │
    │ Event   │
    └────┬────┘
         │
    ┌────▼─────────────────────────┐
    │     Parallel Processing      │
    ├───────────┬─────────┬────────┤
    ▼           ▼         ▼        ▼
Redis      In-Memory   Kafka   Response
Counter    Buffer      Queue   to Client
    │           │         │        │
    │           │         │        ▼
    │           │         │    [200 OK]
    │           │         │
    │           ▼         ▼
    │      [Batch]    [Stream]
    │      100 events  Process
    │           │         │
    │           ▼         ▼
    │      PostgreSQL  Analytics
    │      (Bulk Save) Pipeline
    │
    ▼
[Real-time Metrics]
• Dashboard updates
• Budget tracking
• Performance KPIs
```

## 💰 Campaign Management Flow

```
Advertiser Dashboard
         │
         ▼
  [Campaign Request]
  • Name & Budget
  • Targeting rules
  • Bid strategy
  • Schedule
         │
         ▼
    API Gateway
         │
         ▼
  Campaign Service
         │
    ┌────┴──────────────────┐
    │   Validation          │
    │   • Budget > 0        │
    │   • Valid dates       │
    │   • Targeting valid   │
    └────┬──────────────────┘
         │
    [Success]
         │
    ┌────▼──────────────────────────┐
    │     Parallel Operations       │
    ├─────────┬──────────┬──────────┤
    ▼         ▼          ▼          ▼
PostgreSQL  Redis      Kafka    Response
(Persist)   (Cache)    (Event)  (201 Created)
    │         │          │          │
    │         │          │          ▼
    │         │          │     [Campaign ID]
    │         │          │
    │         ▼          ▼
    │    [Hot Data]   [Notify]
    │    • Budget     • DSPs
    │    • Status     • Reports
    │    • Targeting  • Billing
    │
    ▼
[Persistent Storage]
• Full history
• Audit trail
• Compliance
```

## 🔄 Data Synchronization

```
┌──────────────────────────────────────────────────────┐
│                  SYNC PATTERNS                       │
├──────────────────────────────────────────────────────┤
│                                                      │
│  PostgreSQL ──[Write-through]──▶ Redis              │
│      ↑                             ↓                │
│      │                        [TTL Expiry]          │
│      │                             ↓                │
│      └──────[Lazy Load]────── Cache Miss           │
│                                                      │
│  Kafka ──[Consumer Groups]──▶ Multiple Services     │
│      ↑                                              │
│      │                                              │
│   Events ──[Producer]──▶ Topics                     │
│                                                      │
│  Redis ──[Pub/Sub]──▶ Real-time Updates            │
│                                                      │
└──────────────────────────────────────────────────────┘
```

## 🚦 Service Communication Matrix

| From ↓ / To → | Auction Engine | Campaign Service | Tracking Service | Redis | Kafka | PostgreSQL |
|----------------|---------------|------------------|------------------|-------|-------|------------|
| **Auction Engine** | - | Sync (HTTP) | - | Direct | Async | - |
| **Campaign Service** | - | - | - | Direct | Async | Direct |
| **Tracking Service** | - | Sync (HTTP) | - | Direct | Async | Batch |
| **Redis** | - | - | - | - | - | - |
| **Kafka** | Consume | Consume | Consume | - | - | - |
| **PostgreSQL** | Read | Read/Write | Write | - | - | - |

## 🔐 Security & Monitoring Layer

```
┌─────────────────────────────────────────────────────────┐
│                  OBSERVABILITY                         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Prometheus ◀──[Metrics]──── All Services              │
│      │                                                  │
│      ▼                                                  │
│  Grafana ──[Dashboards]──▶ Operations Team             │
│                                                         │
│  Logrus ──[Structured Logs]──▶ Log Aggregator          │
│                                                         │
│  Health Checks ──[Liveness]──▶ Load Balancer           │
│                                                         │
│  Circuit Breakers ──[Protection]──▶ Downstream         │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## 📊 Performance Optimization Points

```
[Caching Strategy]
L1: In-Memory (Goroutine local) - <1μs
L2: Redis (Shared cache) - <1ms  
L3: PostgreSQL (Source) - <10ms

[Concurrency Model]
• Worker Pools: 100 goroutines
• Channel Buffers: 10,000 events
• Batch Processing: 100 records
• Connection Pools: 25 DB, 10 Redis

[Rate Limiting]
• Per endpoint: 1000 RPS
• Per client: 100 RPS
• Circuit breaker: 50% error rate
• Retry policy: 3 attempts
```

## 🎯 Key Design Decisions

1. **Why Separate Services?**
   - Independent scaling
   - Fault isolation
   - Team ownership
   - Deploy independently

2. **Why Redis + PostgreSQL?**
   - Redis: Speed for real-time operations
   - PostgreSQL: ACID compliance for financial data

3. **Why Kafka?**
   - Decouples producers/consumers
   - Handles backpressure
   - Enables replay
   - Horizontal scaling

4. **Why Go + Gin?**
   - High concurrency
   - Low latency
   - Simple deployment
   - Strong typing

This architecture handles 1000+ RPS with <100ms latency while maintaining data consistency and providing real-time analytics.