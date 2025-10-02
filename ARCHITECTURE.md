# System Architecture & Flow Diagrams

## 📊 Complete System Overview (Mermaid)

```mermaid
graph TB
    subgraph "External Clients"
        P[Publishers]
        A[Advertisers]
        AN[Analytics]
        M[Monitoring]
    end
    
    subgraph "API Gateway"
        LB[Load Balancer]
        GIN[Gin HTTP Router]
    end
    
    subgraph "Business Logic"
        AE[Auction Engine]
        CS[Campaign Service]
        TS[Tracking Service]
        AS[Analytics Service]
    end
    
    subgraph "Data Layer"
        R[(Redis<br/>Cache)]
        K[Kafka<br/>Streams]
        PG[(PostgreSQL<br/>Database)]
    end
    
    P -->|Bid Requests| LB
    A -->|Campaign Mgmt| LB
    AN -->|Get Metrics| LB
    M -->|Health Checks| LB
    
    LB --> GIN
    
    GIN -->|/bid-request| AE
    GIN -->|/campaigns| CS
    GIN -->|/track| TS
    GIN -->|/metrics| AS
    
    AE --> R
    AE --> K
    AE --> CS
    
    CS --> R
    CS --> PG
    CS --> K
    
    TS --> R
    TS --> K
    TS --> PG
    
    AS --> R
    AS --> PG
    
    style P fill:#e1f5fe
    style A fill:#e1f5fe
    style AN fill:#e1f5fe
    style M fill:#e1f5fe
    style AE fill:#fff3e0
    style CS fill:#fff3e0
    style TS fill:#fff3e0
    style AS fill:#fff3e0
    style R fill:#ffebee
    style K fill:#f3e5f5
    style PG fill:#e8f5e9
```

## 🔄 Bid Request Sequence Diagram

```mermaid
sequenceDiagram
    participant P as Publisher
    participant API as API Gateway
    participant AE as Auction Engine
    participant CS as Campaign Service
    participant R as Redis
    participant K as Kafka
    
    P->>API: Bid Request
    API->>AE: Route to Auction
    
    AE->>CS: Fetch Active Campaigns
    CS->>R: Check Budgets
    R-->>CS: Budget Status
    CS-->>AE: Eligible Campaigns
    
    AE->>AE: Evaluate Targeting
    AE->>R: Check Frequency Caps
    R-->>AE: Cap Status
    
    AE->>AE: Calculate Bids
    AE->>AE: Run Auction<br/>(Second Price)
    
    AE->>R: Decrement Budget
    AE->>K: Log Event
    
    AE-->>API: Winning Ad
    API-->>P: Ad Response
    
    Note over K: Async Processing
    K->>K: Analytics
    K->>K: Reporting
    K->>K: Billing
```

## 📈 Event Processing Flow

```mermaid
flowchart LR
    subgraph "Event Source"
        U[User Action]
    end
    
    subgraph "API Layer"
        E[Event Endpoint]
        V[Validation]
    end
    
    subgraph "Processing"
        B[Buffer]
        BP[Batch<br/>Processor]
    end
    
    subgraph "Storage"
        RC[Redis<br/>Counter]
        KQ[Kafka<br/>Queue]
        PG[(PostgreSQL)]
    end
    
    subgraph "Consumers"
        RT[Real-time<br/>Metrics]
        AN[Analytics<br/>Pipeline]
        RP[Reports]
    end
    
    U -->|Click/Impression| E
    E --> V
    V --> B
    V --> RC
    V --> KQ
    
    B -->|100 events| BP
    BP --> PG
    
    RC --> RT
    KQ --> AN
    KQ --> RP
    
    style U fill:#e3f2fd
    style RC fill:#ffebee
    style KQ fill:#f3e5f5
    style PG fill:#e8f5e9
```

## 💰 Campaign Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Draft: Create Campaign
    Draft --> Active: Start Date Reached
    Draft --> Cancelled: Cancel
    
    Active --> Paused: Pause
    Active --> Completed: End Date Reached
    Active --> Exhausted: Budget Depleted
    
    Paused --> Active: Resume
    Paused --> Cancelled: Cancel
    
    Exhausted --> Active: Budget Added
    Exhausted --> Completed: End Date
    
    Completed --> [*]
    Cancelled --> [*]
    
    note right of Active
        Real-time bidding
        Budget tracking
        Performance monitoring
    end note
    
    note right of Exhausted
        No bidding
        Awaiting budget
    end note
```

## 🏗️ Component Interaction Matrix

```mermaid
graph LR
    subgraph "Synchronous Calls"
        AE1[Auction Engine]
        CS1[Campaign Service]
        AE1 -.->|HTTP/gRPC| CS1
    end
    
    subgraph "Async Events"
        AE2[Auction Engine]
        K2[Kafka]
        AN2[Analytics]
        AE2 -->|Publish| K2
        K2 -->|Subscribe| AN2
    end
    
    subgraph "Direct Access"
        CS3[Campaign Service]
        R3[Redis]
        PG3[PostgreSQL]
        CS3 -->|Read/Write| R3
        CS3 -->|Read/Write| PG3
    end
```

## 🚦 Performance Architecture

```mermaid
graph TD
    subgraph "Caching Layers"
        L1[L1: In-Memory<br/>< 1μs]
        L2[L2: Redis<br/>< 1ms]
        L3[L3: PostgreSQL<br/>< 10ms]
    end
    
    subgraph "Concurrency"
        WP[Worker Pool<br/>100 goroutines]
        CH[Channels<br/>10k buffer]
        BP[Batch Size<br/>100 records]
    end
    
    subgraph "Protection"
        RL[Rate Limiter<br/>1000 RPS]
        CB[Circuit Breaker<br/>50% error rate]
        RT[Retry Logic<br/>3 attempts]
    end
    
    L1 --> L2
    L2 --> L3
    
    WP --> CH
    CH --> BP
    
    RL --> CB
    CB --> RT
    
    style L1 fill:#e8f5e9
    style L2 fill:#fff3e0
    style L3 fill:#ffebee
```

## 📊 Data Flow Architecture

```mermaid
flowchart TB
    subgraph "Write Path"
        W[Write Request]
        WV[Validation]
        WT[Transaction]
        WPG[(PostgreSQL)]
        WR[Redis Update]
        WK[Kafka Event]
    end
    
    subgraph "Read Path"
        R[Read Request]
        RC[Redis Cache]
        RM[Cache Miss]
        RPG[(PostgreSQL)]
        RU[Cache Update]
    end
    
    W --> WV
    WV --> WT
    WT --> WPG
    WT --> WR
    WT --> WK
    
    R --> RC
    RC -->|Hit| R
    RC -->|Miss| RM
    RM --> RPG
    RPG --> RU
    RU --> RC
    RC --> R
    
    style WPG fill:#e8f5e9
    style RPG fill:#e8f5e9
    style WR fill:#ffebee
    style RC fill:#ffebee
```

## 📊 Complete System Overview (ASCII)

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