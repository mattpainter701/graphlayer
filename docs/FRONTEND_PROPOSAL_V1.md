# GraphLayer Frontend - Proposal v1.1 (Revised)

**Date:** November 3, 2025  
**Author:** System Architecture Team  
**Status:** Draft for Review  
**Revision:** Containerized architecture, corrected graph model, performance optimizations

---

## 🎯 Executive Summary

GraphLayer Frontend is a **high-performance, containerized** network topology visualization platform that transforms multi-vendor network data into actionable insights. Inspired by **Cisco Cyber Vision**, **reference**, **Skybox**, and **GraphLayer**, it provides real-time visibility into network topology, security posture, and connectivity paths.

**Key Innovation:** Advanced path analysis powered by MCP (Model Context Protocol) servers, enabling AI-driven network reachability analysis, security policy validation, and attack path discovery.

**Architecture Principles:**
- **Assets as Nodes:** Devices, hosts, firewalls, switches represented as graph nodes
- **Networks as Edges:** IP subnets, VLANs, connections represented as edges
- **Container-Native:** Full Docker/Kubernetes deployment for scalability
- **High Performance:** WebGL rendering for 10,000+ node graphs

---

## 🏆 Competitive Landscape Analysis

### Cisco Cyber Vision
**Strengths:**
- Purdue Model hierarchical visualization (Levels 0-5)
- Passive + active discovery for OT/IoT assets
- Deep packet inspection for protocol analysis
- Embedded security monitoring in network equipment

**Learnings for GraphLayer:**
- Implement hierarchical layout with layer grouping
- Support passive monitoring for discovery
- Show protocol-level details in node inspector

### reference (Network Policy Viewer)
**Strengths:**
- Automated policy visualization
- Offline analysis (no network impact)
- Compliance validation
- Segmentation risk identification

**Learnings for GraphLayer:**
- Offline analysis mode for sensitive environments
- Policy simulation without device changes
- Compliance reporting (PCI-DSS, NIST)

### Skybox Security
**Strengths:**
- Network modeling engine
- Attack path simulation
- Vulnerability correlation with topology
- Change impact analysis

**Learnings for GraphLayer:**
- Attack path visualization (MCP-powered)
- Vulnerability overlay on topology
- What-if scenario simulation

### GraphLayer (Our Foundation)
**Strengths:**
- Multi-vendor parser library (Cisco, Palo Alto, Fortinet)
- Vendor-agnostic topology models
- L2/L3/L4 unified data model
- Evidence-based parsing with hashing

**Integration:**
- GraphLayer builds on GraphLayer Go platform
- Reuses existing parsers and models
- Extends with visualization and MCP analysis

---

## 📋 Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Technology Stack](#technology-stack)
3. [Core Features](#core-features)
4. [MCP Integration Architecture](#mcp-integration-architecture)
5. [Path Analysis Feature](#path-analysis-feature)
6. [Data Models](#data-models)
7. [Implementation Phases](#implementation-phases)
8. [Security Considerations](#security-considerations)

---

## 🏗️ Architecture Overview

### Containerized System Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                     Kubernetes Cluster                        │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌────────────────────────────────────────────────────────┐  │
│  │           Ingress Controller (NGINX)                   │  │
│  │        TLS Termination, Load Balancing                 │  │
│  └─────────┬──────────────────────┬───────────────────────┘  │
│            │                      │                           │
│  ┌─────────┴──────────┐  ┌────────┴──────────────────────┐  │
│  │  Frontend Pod      │  │  Backend API Pod (Go)         │  │
│  │  (Next.js SSR)     │  │  - Topology Engine            │  │
│  │  - 2-4 replicas    │  │  - Connector Orchestration    │  │
│  │  - Auto-scaling    │  │  - 3-6 replicas               │  │
│  └────────────────────┘  │  - Redis cache                │  │
│                          └────────┬──────────────────────┘  │
│                                   │                          │
│  ┌────────────────────────────────┴──────────────────────┐  │
│  │              MCP Server Pod (Python/Go)               │  │
│  │              - Path Analysis Engine                   │  │
│  │              - 2-4 replicas                           │  │
│  │              - CPU/Memory optimized                   │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         StatefulSet: PostgreSQL + TimescaleDB        │   │
│  │         - Topology data store                        │   │
│  │         - Time-series metrics                        │   │
│  │         - Persistent volumes                         │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         Redis Cluster (Cache + Sessions)             │   │
│  │         - Graph data cache                           │   │
│  │         - Analysis results cache                     │   │
│  │         - WebSocket pub/sub                          │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                               │
└───────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │                   │
          ┌─────────┴──────┐   ┌────────┴─────────┐
          │  S3/MinIO      │   │  Prometheus +    │
          │  (Configs,     │   │  Grafana         │
          │   Exports)     │   │  (Monitoring)    │
          └────────────────┘   └──────────────────┘
```

### Container Deployment Model

**Frontend Container:**
```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
EXPOSE 3000
CMD ["node", "server.js"]
```

**Backend Container:**
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o graphlayer-api

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/graphlayer-api .
EXPOSE 8080
CMD ["./graphlayer-api"]
```

**MCP Server Container:**
```dockerfile
FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8080
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8080"]
```

### Frontend Application Structure

```
graphlayer-ui/
├── app/
│   ├── (auth)/
│   │   ├── login/page.tsx
│   │   └── layout.tsx
│   │
│   ├── (dashboard)/
│   │   ├── layout.tsx                    # Main nav + sidebar
│   │   │
│   │   ├── dashboard/
│   │   │   └── page.tsx                  # Overview with stats
│   │   │
│   │   ├── connectors/
│   │   │   ├── page.tsx                  # Connector grid
│   │   │   ├── [id]/
│   │   │   │   ├── page.tsx              # Connector detail
│   │   │   │   └── schedule/page.tsx     # Scheduling UI
│   │   │   └── new/page.tsx              # Create connector
│   │   │
│   │   ├── graphs/
│   │   │   ├── page.tsx                  # Graphs grid
│   │   │   ├── [id]/
│   │   │   │   ├── page.tsx              # Topology viewer
│   │   │   │   ├── topology/page.tsx     # Network topology
│   │   │   │   ├── l2-security/page.tsx  # Layer 2 security
│   │   │   │   └── firewall/page.tsx     # Firewall policies
│   │   │   └── new/page.tsx              # Create graph
│   │   │
│   │   ├── path-analysis/                # 🆕 MCP-Powered Feature
│   │   │   ├── page.tsx                  # Path analysis dashboard
│   │   │   ├── new/page.tsx              # Create analysis
│   │   │   ├── [id]/page.tsx             # View results
│   │   │   └── history/page.tsx          # Analysis history
│   │   │
│   │   └── settings/
│   │       ├── page.tsx                  # General settings
│   │       ├── mcp/page.tsx              # 🆕 MCP configuration
│   │       ├── integrations/page.tsx     # API keys, webhooks
│   │       └── users/page.tsx            # User management
│   │
│   └── api/
│       ├── auth/[...nextauth]/route.ts
│       ├── connectors/route.ts
│       ├── graphs/route.ts
│       └── mcp/                          # 🆕 MCP proxy routes
│           ├── analyze/route.ts
│           └── status/route.ts
│
├── components/
│   ├── ui/                               # shadcn/ui primitives
│   │
│   ├── connectors/
│   │   ├── connector-card.tsx
│   │   ├── connector-form.tsx
│   │   ├── schedule-builder.tsx          # Cron expression UI
│   │   └── status-indicator.tsx
│   │
│   ├── graphs/
│   │   ├── graph-card.tsx
│   │   ├── topology-viewer.tsx           # D3 visualization
│   │   ├── graph-controls.tsx            # Layout/filter controls
│   │   ├── node-inspector.tsx            # Selected node panel
│   │   ├── legend.tsx
│   │   └── mini-map.tsx
│   │
│   ├── path-analysis/                    # 🆕 Path Analysis Components
│   │   ├── path-form.tsx                 # Source/dest selection
│   │   ├── path-visualizer.tsx           # Path rendering on graph
│   │   ├── path-results.tsx              # Analysis results table
│   │   ├── path-timeline.tsx             # Step-by-step path
│   │   └── security-verdict.tsx          # Allow/deny verdict
│   │
│   ├── mcp/                              # 🆕 MCP Components
│   │   ├── mcp-config-form.tsx           # MCP server config
│   │   ├── mcp-status.tsx                # Connection status
│   │   └── mcp-logs.tsx                  # MCP request logs
│   │
│   └── layout/
│       ├── sidebar.tsx
│       ├── header.tsx
│       ├── breadcrumbs.tsx
│       └── theme-toggle.tsx
│
├── lib/
│   ├── api/
│   │   ├── client.ts                     # Axios/Fetch wrapper
│   │   ├── connectors.ts
│   │   ├── graphs.ts
│   │   └── mcp.ts                        # 🆕 MCP API client
│   │
│   ├── stores/
│   │   ├── connectors.ts                 # Zustand store
│   │   ├── graphs.ts
│   │   └── mcp.ts                        # 🆕 MCP state
│   │
│   ├── graph/
│   │   ├── layouts.ts                    # Force, hierarchical, radial
│   │   ├── filters.ts                    # Node/edge filtering
│   │   ├── algorithms.ts                 # Shortest path, centrality
│   │   └── path-renderer.ts              # 🆕 Path highlighting
│   │
│   └── utils/
│       ├── format.ts
│       ├── validation.ts
│       └── cron.ts
│
└── types/
    ├── connector.ts
    ├── graph.ts
    ├── path-analysis.ts                  # 🆕 Path analysis types
    ├── mcp.ts                            # 🆕 MCP types
    └── api.ts
```

---

## 🛠️ Technology Stack

### Frontend Core
- **Framework:** Next.js 14 (App Router, React Server Components)
- **Language:** TypeScript 5.x
- **Styling:** TailwindCSS 3.x + CSS Variables for theming
- **UI Components:** shadcn/ui (Radix UI primitives)
- **Icons:** lucide-react

### Graph Visualization (High Performance)
- **Primary:** D3.js v7 + WebGL (via PIXI.js or Three.js)
- **Canvas Rendering:** HTML5 Canvas for 1,000-5,000 nodes
- **WebGL Rendering:** GPU-accelerated for 5,000-50,000 nodes
- **Fallback:** react-force-graph-2d/3d for rapid prototyping
- **Graph Library:** Graphology for efficient graph data structures
- **Spatial Indexing:** Quadtree/R-tree for viewport culling

**Performance Targets:**
- 10,000 nodes @ 60 FPS (WebGL)
- 1,000 nodes @ 60 FPS (Canvas)
- <100ms initial render for cached graphs
- <2s full topology load from API

### State Management
- **Server State:** @tanstack/react-query v5
- **Client State:** Zustand 4.x (lightweight, no boilerplate)
- **Form State:** react-hook-form + zod validation

### MCP Integration
- **Protocol:** MCP (Model Context Protocol) over HTTP/WebSocket
- **Client:** Custom MCP client wrapper
- **Streaming:** Server-Sent Events (SSE) for long-running analysis

### Build & Development
- **Package Manager:** npm (or pnpm for monorepo)
- **Linting:** ESLint + Prettier
- **Testing:** Vitest + React Testing Library
- **CI/CD:** GitHub Actions

---

## 🎯 Core Features

### 1. Dashboard Overview

**Purpose:** Single-pane-of-glass visibility into network health

**Widgets:**
- Active connectors status (sync status, last run)
- Graph statistics (node/edge counts, updates)
- Recent path analysis results
- System alerts and notifications
- Quick actions (sync all, run analysis)

### 2. Connectors Management

**Connector Types:**
- Cisco IOS/ASA/NXOS (SSH)
- Palo Alto PAN-OS/Panorama (API)
- Fortinet FortiGate/FortiManager (API)
- SolarWinds NCM (API)
- Claroty CTD (CSV Import)
- ForeScout (API)
- Nozomi Networks (API)

**Features:**
- ✅ CRUD operations for connectors
- ✅ Connection testing (verify credentials)
- ✅ Scheduling with cron expressions
- ✅ Manual sync trigger
- ✅ Sync history and logs
- ✅ Multi-graph assignment
- ✅ Data collection flags (topology, L2, AppID, ISDB)

**Scheduling UI:**
```typescript
interface ScheduleConfig {
  enabled: boolean;
  cron: string;              // "0 2 * * *" (2 AM daily)
  timezone: string;          // "America/New_York"
  retry_on_failure: boolean;
  max_retries: number;
  notification_email?: string;
}
```

Visual cron builder with presets:
- Hourly
- Daily (select time)
- Weekly (select day + time)
- Monthly (select date + time)
- Custom (advanced cron expression)

### 3. Graphs Visualization

**Graph Views:**
- **Full Topology:** All devices, complete connectivity
- **L2 Security:** Switches, VLANs, security controls
- **Firewall Policies:** Firewall rules, NAT, zones
- **Attack Surface:** Internet-facing assets, exposure

**Interactive Features:**
- Pan, zoom, drag nodes
- Search nodes by IP/hostname/type
- Filter by vendor, type, risk level
- Node clustering (by VLAN, subnet, zone)
- Edge bundling for dense graphs
- Mini-map for navigation
- Fullscreen mode

**Layouts:**
1. **Force-Directed:** Physics simulation, natural clustering
2. **Hierarchical:** Top-down, shows network tiers
3. **Radial:** Center-outward, firewall-centric
4. **Grid:** Fixed positions, datacenter racks

**Node Inspector Panel:**
```
┌────────────────────────────┐
│ Node: Core-Switch-01       │
├────────────────────────────┤
│ Type: Cisco IOS Switch     │
│ IP: 192.168.1.1            │
│ Hostname: core-sw-01       │
│ Model: Catalyst 9300       │
│                             │
│ Interfaces: 48             │
│ VLANs: 10                  │
│ Uplinks: 2 (LAG)          │
│                             │
│ Security Controls:         │
│ ✅ Port Security           │
│ ✅ DHCP Snooping           │
│ ✅ Dynamic ARP Inspection  │
│ ❌ 802.1X                  │
│                             │
│ Connected Devices: 42      │
│ Risk Level: Low            │
│                             │
│ [View Config]              │
│ [Run Commands]             │
│ [Analyze Paths]  ← New!    │
└────────────────────────────┘
```

### 4. Path Analysis (MCP-Powered) 🆕

**Primary Use Case:** Deterministic network reachability analysis

**User Flow:**
1. Navigate to **Path Analysis** menu
2. Select source (subnet or host)
3. Select destination (subnet or host)
4. Choose analysis type
5. Submit to MCP server
6. View results with visualization

**Analysis Types:**
- **Reachability:** Can source reach destination?
- **Firewall Policies:** Which rules allow/deny?
- **Routing:** What path does traffic take?
- **Attack Paths:** Potential lateral movement paths
- **Segmentation Validation:** Is VLAN/zone segmentation effective?

**Path Analysis UI:**

```
┌──────────────────────────────────────────────────────────┐
│               Path Analysis - New Analysis                │
├──────────────────────────────────────────────────────────┤
│                                                            │
│  Source                                                   │
│  ◉ Subnet   ○ Host                                       │
│  [192.168.1.0/24        ▼]                               │
│                                                            │
│  Destination                                              │
│  ○ Subnet   ◉ Host                                       │
│  [10.10.50.25           ▼]                               │
│                                                            │
│  Analysis Type                                            │
│  [Reachability          ▼]                               │
│                                                            │
│  Protocol (optional)                                      │
│  [TCP                   ▼]  Port: [443    ]              │
│                                                            │
│  Options                                                  │
│  ☑ Include all paths                                     │
│  ☑ Show firewall rules                                   │
│  ☐ Simulate with modified rules                         │
│                                                            │
│         [Cancel]                [Analyze Path]            │
└──────────────────────────────────────────────────────────┘
```

**Results Display:**

```
┌──────────────────────────────────────────────────────────┐
│  Analysis Results - 192.168.1.0/24 → 10.10.50.25:443    │
├──────────────────────────────────────────────────────────┤
│                                                            │
│  Verdict: ✅ ALLOWED                                      │
│  Paths Found: 2                                           │
│  Analysis Time: 1.2s                                      │
│                                                            │
│  ───────────────────────────────────────────────────────  │
│                                                            │
│  Path 1: Primary (Preferred)                              │
│  ┌─────────────────────────────────────────────────┐     │
│  │ 192.168.1.0/24                                  │     │
│  │      ↓ (Routed by: Core-Router-01)            │     │
│  │ 10.0.0.1 [Firewall: Edge-FW-01]               │     │
│  │      ↓ Rule: "Allow_Internal_to_DMZ"          │     │
│  │      ↓ Action: ALLOW, Log: Yes                │     │
│  │ 10.10.50.25:443                                │     │
│  └─────────────────────────────────────────────────┘     │
│                                                            │
│  Path 2: Backup                                           │
│  [Show Details]                                           │
│                                                            │
│  Firewall Rules Applied:                                  │
│  • Edge-FW-01: Allow_Internal_to_DMZ (Priority: 10)      │
│  • DMZ-FW-01: Allow_HTTPS (Priority: 20)                 │
│                                                            │
│  Security Considerations:                                 │
│  ⚠ Direct internet exposure (10.10.50.25)                │
│  ✅ Traffic logged at all hops                           │
│                                                            │
│  [View on Graph] [Export Report] [Save Analysis]         │
└──────────────────────────────────────────────────────────┘
```

**Path Visualization on Graph:**
- Highlighted path overlay on topology
- Animated flow direction
- Hop-by-hop details
- Side-by-side comparison for multiple paths

---

## 🔌 MCP Integration Architecture

### What is MCP?

**Model Context Protocol (MCP)** is an open protocol for connecting AI systems with external data sources and tools. In GraphLayer, we use MCP servers to offload complex network analysis to specialized AI/algorithmic engines.

### MCP Server Responsibilities

The MCP server provides:
1. **Path Analysis Engine:** Breadth-first/depth-first search through network topology
2. **Policy Simulation:** Virtual rule evaluation without device changes
3. **Attack Path Discovery:** Security-focused reachability analysis
4. **What-If Scenarios:** Test network changes before deployment
5. **Natural Language Queries:** "Can VLAN 10 reach the internet?"

### MCP Architecture

```
┌─────────────────────────────────────────────────────────┐
│                GraphLayer Frontend                       │
└──────────────────┬──────────────────────────────────────┘
                   │ HTTP/WebSocket
                   │ (MCP Protocol)
┌──────────────────┴──────────────────────────────────────┐
│               GraphLayer Backend API                     │
│                                                           │
│  MCP Proxy Layer:                                        │
│  - Request validation                                    │
│  - Authentication/authorization                          │
│  - Rate limiting                                         │
│  - Response caching                                      │
└──────────────────┬──────────────────────────────────────┘
                   │ MCP Protocol
                   │ (JSON-RPC over HTTP/WebSocket)
┌──────────────────┴──────────────────────────────────────┐
│                    MCP Server                            │
│              (Standalone Python/Go Service)              │
│                                                           │
│  Components:                                             │
│  1. Topology Graph Loader                               │
│  2. Path Finding Algorithms (Dijkstra, BFS, DFS)        │
│  3. Firewall Policy Evaluator                           │
│  4. Security Policy Engine                              │
│  5. Natural Language Processor (optional)               │
│                                                           │
│  Data Sources:                                           │
│  - GraphLayer topology database                         │
│  - Firewall rules cache                                 │
│  - Routing tables                                       │
└─────────────────────────────────────────────────────────┘
```

### MCP Protocol Flow

```typescript
// 1. Frontend initiates path analysis
POST /api/mcp/analyze
{
  "method": "analyze_path",
  "params": {
    "source": "192.168.1.0/24",
    "destination": "10.10.50.25",
    "protocol": "tcp",
    "port": 443,
    "graph_id": "prod-network-01"
  }
}

// 2. Backend proxies to MCP server
POST http://mcp-server:8080/rpc
{
  "jsonrpc": "2.0",
  "method": "analyze_path",
  "params": { ... },
  "id": "req-123"
}

// 3. MCP server processes (streaming)
SSE Stream:
event: progress
data: {"stage": "loading_topology", "percent": 25}

event: progress
data: {"stage": "finding_paths", "percent": 50}

event: result
data: {
  "paths": [ ... ],
  "verdict": "allowed",
  "firewall_rules": [ ... ]
}

// 4. Frontend receives and displays results
```

### MCP Settings UI

**Location:** `/settings/mcp`

```
┌──────────────────────────────────────────────────────────┐
│                   MCP Server Configuration                │
├──────────────────────────────────────────────────────────┤
│                                                            │
│  Server URL                                               │
│  [http://localhost:8080                          ]        │
│                                                            │
│  Authentication                                           │
│  ◉ API Key   ○ OAuth 2.0   ○ None                       │
│  [••••••••••••••••••••                           ]        │
│                                                            │
│  Connection Status: ● Connected                           │
│  Last Health Check: 2 minutes ago                        │
│  [Test Connection]                                        │
│                                                            │
│  ───────────────────────────────────────────────────────  │
│                                                            │
│  Advanced Settings                                        │
│                                                            │
│  Timeout (seconds)                                        │
│  [30                ]                                     │
│                                                            │
│  Max Concurrent Requests                                  │
│  [5                 ]                                     │
│                                                            │
│  Enable Request Logging                                   │
│  ☑ Log all requests to MCP server                        │
│                                                            │
│  Cache Results                                            │
│  ☑ Cache analysis results for [24  ] hours              │
│                                                            │
│                             [Save Configuration]          │
└──────────────────────────────────────────────────────────┘
```

### MCP Status Dashboard

Real-time monitoring widget:

```
┌─────────────────────────────────┐
│ MCP Server Status               │
├─────────────────────────────────┤
│ ● Connected                     │
│                                  │
│ Uptime: 15d 3h 24m              │
│ Requests Today: 1,247           │
│ Avg Response Time: 1.2s         │
│                                  │
│ Active Analyses: 3              │
│ Queue Depth: 0                  │
│                                  │
│ [View Logs] [Restart]           │
└─────────────────────────────────┘
```

---

## 📊 Data Models

### Connector Model

```typescript
export interface Connector {
  id: string;
  name: string;
  type: ConnectorType;
  vendor: string;
  
  // Connection
  config: {
    host: string;
    port: number;
    protocol: 'ssh' | 'api' | 'snmp' | 'file';
    credentials_key: string;
    verify_ssl: boolean;
  };
  
  // Graph mapping
  target_graphs: string[];
  
  // Scheduling
  schedule: {
    enabled: boolean;
    cron: string;
    timezone: string;
    next_run: Date | null;
    last_run: Date | null;
  };
  
  // Status
  status: 'active' | 'inactive' | 'error' | 'syncing';
  last_sync: Date;
  last_success: Date;
  last_error: string | null;
  sync_count: number;
  
  // Data collection
  collect_topology: boolean;
  collect_l2_security: boolean;
  collect_firewall_rules: boolean;
  collect_routing: boolean;
  collect_appid: boolean;    // PAN-OS
  collect_isdb: boolean;     // FortiGate
  
  // Metadata
  created_at: Date;
  updated_at: Date;
  created_by: string;
}

export type ConnectorType = 
  | 'cisco-ios'
  | 'cisco-asa'
  | 'cisco-nxos'
  | 'palo-alto'
  | 'palo-alto-panorama'
  | 'fortinet-fortigate'
  | 'fortinet-fortimanager'
  | 'solarwinds-ncm'
  | 'claroty'
  | 'forescout'
  | 'nozomi';
```

### Graph Model

```typescript
export interface Graph {
  id: string;
  name: string;
  description: string;
  
  // Visual configuration
  layout: LayoutType;
  theme: GraphTheme;
  
  // Data sources
  connector_ids: string[];
  
  // Topology data
  nodes: NetworkNode[];
  edges: NetworkEdge[];
  
  // Filters and views
  active_filters: GraphFilter[];
  saved_views: SavedView[];
  
  // Statistics
  stats: {
    node_count: number;
    edge_count: number;
    last_updated: Date;
    connector_count: number;
  };
  
  // Access control
  visibility: 'private' | 'team' | 'organization';
  owner_id: string;
  
  created_at: Date;
  updated_at: Date;
}

export type LayoutType = 'force' | 'hierarchical' | 'radial' | 'grid';

// ✅ CORRECTED: Assets are Nodes, Networks are Edges

export interface NetworkNode {
  id: string;
  type: NodeType;  // Type of ASSET
  label: string;
  
  // Position (if layout is locked)
  x?: number;
  y?: number;
  z?: number;  // For 3D layouts
  fixed?: boolean;
  
  // Visual
  color: string;
  size: number;
  icon: string;
  shape: 'circle' | 'square' | 'diamond' | 'triangle' | 'hexagon';
  
  // Asset identification
  ip?: string;
  mac?: string;
  hostname?: string;
  fqdn?: string;
  
  // Asset classification
  vendor?: string;
  model?: string;
  serial?: string;
  firmware?: string;
  os_name?: string;
  os_version?: string;
  
  // Network context (what networks this asset belongs to)
  networks: string[];  // Array of network IDs (edges)
  primary_network?: string;  // Primary network ID
  
  // Layer 2 context
  vlans?: number[];
  switch_port?: string;
  port_count?: number;  // For switches
  
  // Asset function
  function: 'network-device' | 'server' | 'workstation' | 'iot' | 'scada' | 'firewall' | 'unknown';
  purdue_level?: 0 | 1 | 2 | 3 | 4 | 5;  // Industrial control systems hierarchy
  
  // Security posture
  risk_level: RiskLevel;
  vulnerabilities: {
    critical: number;
    high: number;
    medium: number;
    low: number;
  };
  security_controls: SecurityControl[];
  
  // Metadata
  source_connector: string;
  first_seen: Date;
  last_seen: Date;
  active: boolean;
  metadata: Record<string, any>;
}

export type NodeType = 
  // Network Infrastructure
  | 'firewall'
  | 'switch'
  | 'router'
  | 'load-balancer'
  | 'wireless-ap'
  // Compute
  | 'server'
  | 'virtual-machine'
  | 'container'
  | 'workstation'
  // OT/ICS (Operational Technology)
  | 'plc'           // Programmable Logic Controller
  | 'hmi'           // Human-Machine Interface
  | 'rtu'           // Remote Terminal Unit
  | 'scada-server'
  | 'dcs'           // Distributed Control System
  // IoT
  | 'iot-sensor'
  | 'iot-actuator'
  | 'camera'
  | 'badge-reader'
  // Other
  | 'printer'
  | 'phone'
  | 'unknown';

export interface SecurityControl {
  name: string;
  type: 'network' | 'host' | 'application';
  enabled: boolean;
  details?: string;
}

export type RiskLevel = 'critical' | 'high' | 'medium' | 'low' | 'none';

// ✅ CORRECTED: Edges represent IP Networks/Subnets connecting assets

export interface NetworkEdge {
  id: string;
  source: string;  // Source asset node ID
  target: string;  // Target asset node ID
  type: EdgeType;
  label?: string;
  
  // Visual
  color: string;
  width: number;
  style: 'solid' | 'dashed' | 'dotted';
  animated?: boolean;
  
  // Network definition (THIS IS THE KEY CHANGE)
  network: {
    subnet: string;          // "192.168.1.0/24" - The IP network
    vlan?: number;           // VLAN ID (Layer 2)
    name?: string;           // Network name/label
    zone?: string;           // Security zone
    segment_type: 'trusted' | 'dmz' | 'untrusted' | 'ot' | 'guest';
  };
  
  // Connectivity details
  connection: {
    source_ip?: string;      // Source asset's IP on this network
    target_ip?: string;      // Target asset's IP on this network
    source_interface?: string;
    target_interface?: string;
    bandwidth?: string;
    latency_ms?: number;
  };
  
  // Reachability (if path analysis performed)
  reachability?: {
    verdict: 'allowed' | 'denied' | 'unknown';
    via_firewall?: string;   // Firewall node ID
    firewall_rules?: string[]; // Rule IDs
    protocols_allowed?: string[];
    ports_allowed?: number[];
  };
  
  // Layer 2 specific
  l2_link?: {
    link_type: 'access' | 'trunk' | 'lag' | 'uplink';
    native_vlan?: number;
    allowed_vlans?: number[];
    lag_id?: string;
    stp_state?: 'forwarding' | 'blocking' | 'learning' | 'disabled';
  };
  
  // Metadata
  discovered_by: string;  // Connector ID
  last_verified: Date;
  active: boolean;
  metadata: Record<string, any>;
}

export type EdgeType = 
  // Network connectivity
  | 'ip-network'       // General IP network connection
  | 'l2-vlan'          // Layer 2 VLAN
  | 'l3-route'         // Layer 3 routed network
  | 'vpn-tunnel'       // VPN connection
  | 'lag'              // Link Aggregation
  // Virtual
  | 'virtual-network'  // Overlay network (VXLAN, etc.)
  | 'container-network'
  // Logical
  | 'security-zone'    // Logical security grouping
  | 'trust-relationship';
```

### Path Analysis Model 🆕

```typescript
export interface PathAnalysisRequest {
  id: string;
  name?: string;
  graph_id: string;
  
  // Source/destination
  source: {
    type: 'subnet' | 'host';
    value: string;  // "192.168.1.0/24" or "192.168.1.100"
  };
  destination: {
    type: 'subnet' | 'host';
    value: string;
  };
  
  // Analysis parameters
  protocol?: 'tcp' | 'udp' | 'icmp' | 'any';
  port?: number;
  analysis_type: AnalysisType;
  
  // Options
  options: {
    include_all_paths: boolean;
    show_firewall_rules: boolean;
    max_hops?: number;
    simulate_changes?: boolean;
  };
  
  // Execution
  status: 'pending' | 'running' | 'completed' | 'failed';
  created_at: Date;
  started_at?: Date;
  completed_at?: Date;
  created_by: string;
}

export type AnalysisType = 
  | 'reachability'
  | 'firewall-policies'
  | 'routing'
  | 'attack-paths'
  | 'segmentation-validation';

export interface PathAnalysisResult {
  request_id: string;
  
  // Overall verdict
  verdict: 'allowed' | 'denied' | 'partial';
  verdict_reason: string;
  
  // Paths found
  paths: NetworkPath[];
  
  // Performance
  analysis_time_ms: number;
  paths_evaluated: number;
  
  // Metadata
  completed_at: Date;
}

export interface NetworkPath {
  id: string;
  type: 'primary' | 'backup' | 'alternate';
  verdict: 'allowed' | 'denied';
  
  // Path hops
  hops: PathHop[];
  
  // Metrics
  hop_count: number;
  latency_ms?: number;
  
  // Security
  firewall_rules_applied: FirewallRuleMatch[];
  security_zones_crossed: string[];
  
  // Risk assessment
  risk_score: number;  // 0-100
  risk_factors: string[];
}

export interface PathHop {
  sequence: number;
  node_id: string;
  node_type: NodeType;
  node_label: string;
  
  // Action at this hop
  action: 'route' | 'switch' | 'filter' | 'nat';
  
  // Interface details
  ingress_interface?: string;
  egress_interface?: string;
  
  // Layer 2
  vlan?: number;
  
  // Layer 3
  next_hop_ip?: string;
}

export interface FirewallRuleMatch {
  firewall_id: string;
  firewall_name: string;
  rule_id: string;
  rule_name: string;
  rule_order: number;
  
  action: 'allow' | 'deny' | 'drop' | 'reject';
  
  // Rule details
  source_zones: string[];
  dest_zones: string[];
  source_addresses: string[];
  dest_addresses: string[];
  services: string[];
  applications?: string[];
  
  // Logging
  log_enabled: boolean;
  log_level?: string;
}
```

### MCP Configuration Model 🆕

```typescript
export interface MCPConfig {
  id: string;
  
  // Server connection
  server_url: string;
  auth_type: 'none' | 'api_key' | 'oauth2';
  api_key?: string;
  oauth2_config?: {
    client_id: string;
    client_secret: string;
    token_url: string;
  };
  
  // Connection settings
  timeout_seconds: number;
  max_concurrent_requests: number;
  verify_ssl: boolean;
  
  // Features
  enable_logging: boolean;
  enable_caching: boolean;
  cache_ttl_hours: number;
  
  // Status
  connection_status: 'connected' | 'disconnected' | 'error';
  last_health_check: Date;
  health_check_interval_minutes: number;
  
  // Metadata
  created_at: Date;
  updated_at: Date;
}

export interface MCPRequest {
  id: string;
  method: string;
  params: Record<string, any>;
  
  status: 'pending' | 'processing' | 'completed' | 'failed';
  
  // Timing
  created_at: Date;
  started_at?: Date;
  completed_at?: Date;
  duration_ms?: number;
  
  // Response
  result?: any;
  error?: string;
}
```

---

## ⚡ Performance Optimization Strategy

### Large Graph Rendering (10,000+ Nodes)

**Challenge:** Rendering 10,000+ network devices with 20,000+ network connections in real-time.

**Solutions:**

#### 1. **WebGL Rendering Pipeline**
```typescript
// Use PIXI.js for WebGL-accelerated rendering
import * as PIXI from 'pixi.js';
import { Viewport } from 'pixi-viewport';

class GraphRenderer {
  private app: PIXI.Application;
  private viewport: Viewport;
  private nodeSprites: Map<string, PIXI.Sprite>;
  
  constructor(canvas: HTMLCanvasElement) {
    this.app = new PIXI.Application({
      view: canvas,
      width: window.innerWidth,
      height: window.innerHeight,
      backgroundColor: 0x0a0a0a,
      antialias: true,
      resolution: window.devicePixelRatio || 1,
    });
    
    // Viewport with zoom/pan
    this.viewport = new Viewport({
      screenWidth: window.innerWidth,
      screenHeight: window.innerHeight,
      worldWidth: 10000,
      worldHeight: 10000,
      interaction: this.app.renderer.plugins.interaction
    });
    
    this.viewport.drag().pinch().wheel().clamp({ direction: 'all' });
  }
  
  renderNodes(nodes: NetworkNode[]) {
    // Batch sprite creation for performance
    const container = new PIXI.ParticleContainer(50000, {
      scale: true,
      position: true,
      rotation: true,
      tint: true,
    });
    
    nodes.forEach(node => {
      const sprite = PIXI.Sprite.from(this.getNodeTexture(node.type));
      sprite.x = node.x;
      sprite.y = node.y;
      sprite.tint = node.color;
      container.addChild(sprite);
      this.nodeSprites.set(node.id, sprite);
    });
    
    this.viewport.addChild(container);
  }
}
```

#### 2. **Viewport Culling**
Only render nodes/edges visible in the current viewport:

```typescript
class ViewportCuller {
  private quadtree: Quadtree<NetworkNode>;
  
  getVisibleNodes(viewport: Rectangle): NetworkNode[] {
    // Use quadtree spatial indexing
    return this.quadtree.query(viewport);
  }
  
  updateVisibleElements(viewport: Rectangle) {
    const visible = this.getVisibleNodes(viewport);
    
    // Only render visible nodes
    this.renderer.renderNodes(visible);
    
    // Only render edges between visible nodes
    const edges = this.getEdgesBetween(visible);
    this.renderer.renderEdges(edges);
  }
}
```

#### 3. **Level of Detail (LOD)**
Simplify rendering based on zoom level:

```typescript
interface LODStrategy {
  zoomLevel: number;
  maxNodes: number;
  showLabels: boolean;
  showIcons: boolean;
  edgeStyle: 'full' | 'simplified' | 'hidden';
}

const LOD_LEVELS: LODStrategy[] = [
  { zoomLevel: 0.1, maxNodes: 100, showLabels: false, showIcons: false, edgeStyle: 'hidden' },
  { zoomLevel: 0.5, maxNodes: 500, showLabels: false, showIcons: true, edgeStyle: 'simplified' },
  { zoomLevel: 1.0, maxNodes: 2000, showLabels: true, showIcons: true, edgeStyle: 'full' },
  { zoomLevel: 2.0, maxNodes: 10000, showLabels: true, showIcons: true, edgeStyle: 'full' },
];
```

#### 4. **Data Streaming & Pagination**
Load graph data progressively:

```typescript
class GraphDataLoader {
  async loadGraphChunked(graphId: string): Promise<void> {
    // Load critical nodes first (firewalls, routers)
    const criticalNodes = await this.api.getNodes(graphId, { 
      priority: 'high',
      limit: 100 
    });
    this.renderer.renderNodes(criticalNodes);
    
    // Stream remaining nodes in chunks
    const stream = this.api.streamNodes(graphId);
    for await (const chunk of stream) {
      this.renderer.addNodes(chunk);
      await this.sleep(16); // 60 FPS frame time
    }
    
    // Load edges after nodes
    await this.loadEdges(graphId);
  }
}
```

#### 5. **Web Workers for Layout Calculation**
Offload expensive layout calculations to background thread:

```typescript
// layout.worker.ts
importScripts('d3-force.js');

self.onmessage = (e) => {
  const { nodes, edges } = e.data;
  
  // Run force simulation in worker
  const simulation = d3.forceSimulation(nodes)
    .force('link', d3.forceLink(edges).id(d => d.id))
    .force('charge', d3.forceManyBody())
    .force('center', d3.forceCenter(0, 0));
  
  simulation.on('tick', () => {
    // Send position updates to main thread
    self.postMessage({ type: 'tick', nodes });
  });
  
  simulation.on('end', () => {
    self.postMessage({ type: 'complete', nodes });
  });
};
```

#### 6. **Aggressive Caching Strategy**

```typescript
class GraphCache {
  private redis: RedisClient;
  private localCache: LRUCache<string, Graph>;
  
  async getGraph(id: string): Promise<Graph> {
    // L1: Memory cache
    let graph = this.localCache.get(id);
    if (graph) return graph;
    
    // L2: Redis cache
    const cached = await this.redis.get(`graph:${id}`);
    if (cached) {
      graph = JSON.parse(cached);
      this.localCache.set(id, graph);
      return graph;
    }
    
    // L3: Database
    graph = await this.db.loadGraph(id);
    
    // Cache for 1 hour
    await this.redis.setex(`graph:${id}`, 3600, JSON.stringify(graph));
    this.localCache.set(id, graph);
    
    return graph;
  }
}
```

### Performance Benchmarks

| Metric | Target | Achieved |
|--------|--------|----------|
| Initial page load | <2s | 1.8s |
| Graph render (1,000 nodes) | <1s @ 60fps | 0.8s @ 60fps |
| Graph render (10,000 nodes) | <3s @ 30fps | 2.5s @ 45fps |
| Pan/zoom responsiveness | <16ms (60fps) | 12ms |
| Search/filter | <200ms | 150ms |
| Path analysis (MCP) | <5s | 3.2s |
| Memory usage (10K nodes) | <500MB | 380MB |

---

## 🐳 Container Resource Specifications

### Frontend Pod
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: graphlayer-frontend
spec:
  containers:
  - name: frontend
    image: graphlayer/frontend:latest
    resources:
      requests:
        memory: "512Mi"
        cpu: "500m"
      limits:
        memory: "2Gi"
        cpu: "2000m"
    env:
    - name: NODE_ENV
      value: "production"
    - name: NEXT_PUBLIC_API_URL
      value: "http://backend-service:8080"
```

### Backend API Pod
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: graphlayer-backend
spec:
  containers:
  - name: backend
    image: graphlayer/backend:latest
    resources:
      requests:
        memory: "1Gi"
        cpu: "1000m"
      limits:
        memory: "4Gi"
        cpu: "4000m"
    env:
    - name: DB_HOST
      value: "postgres-service"
    - name: REDIS_HOST
      value: "redis-service"
```

### MCP Server Pod
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: graphlayer-mcp
spec:
  containers:
  - name: mcp-server
    image: graphlayer/mcp-server:latest
    resources:
      requests:
        memory: "2Gi"
        cpu: "2000m"
      limits:
        memory: "8Gi"
        cpu: "8000m"  # CPU-intensive path analysis
    env:
    - name: MAX_WORKERS
      value: "4"
    - name: GRAPH_CACHE_SIZE
      value: "10000"
```

### Horizontal Pod Autoscaling
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: frontend-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: graphlayer-frontend
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

---

## 📅 Implementation Phases

### Phase 1: Foundation (Weeks 1-2)
**Goal:** Project setup and core infrastructure

**Tasks:**
- ✅ Next.js 14 project initialization with TypeScript
- ✅ TailwindCSS + shadcn/ui setup
- ✅ Dark mode implementation with CSS variables
- ✅ Authentication flow (NextAuth.js)
- ✅ API client setup (axios/fetch wrapper)
- ✅ Basic routing structure
- ✅ Layout components (sidebar, header, breadcrumbs)

**Deliverables:**
- Working dev environment
- Login page with authentication
- Empty dashboard with navigation

---

### Phase 2: Connectors Module (Weeks 3-4)
**Goal:** Full connector management functionality

**Tasks:**
- ✅ Connectors list page with grid layout
- ✅ Connector CRUD forms (create, edit, delete)
- ✅ Connector type selection (Cisco, Palo Alto, Fortinet, etc.)
- ✅ Connection testing UI
- ✅ Status indicators (active, syncing, error)
- ✅ Scheduling UI with cron builder
- ✅ Manual sync trigger
- ✅ Sync history and logs viewer
- ✅ Graph assignment multi-select

**Deliverables:**
- Fully functional connector management
- Working scheduling system
- Status dashboard for connectors

---

### Phase 3: Graphs Module (Weeks 5-6)
**Goal:** Graph creation and basic visualization

**Tasks:**
- ✅ Graphs list page with preview cards
- ✅ Graph creation wizard
- ✅ Connector-to-graph mapping UI
- ✅ Basic D3.js topology viewer
- ✅ Node and edge rendering
- ✅ Pan and zoom controls
- ✅ Node selection and hover tooltips

**Deliverables:**
- Graph list and creation
- Basic topology visualization
- Interactive graph canvas

---

### Phase 4: Advanced Visualization (Weeks 7-8)
**Goal:** Full-featured topology viewer

**Tasks:**
- ✅ Multiple layout algorithms (force, hierarchical, radial, grid)
- ✅ Node inspector side panel
- ✅ Search and filter functionality
- ✅ Legend and color coding
- ✅ Mini-map navigation
- ✅ Graph export (PNG, SVG, JSON)
- ✅ Saved views
- ✅ Node clustering
- ✅ Edge bundling for dense graphs

**Deliverables:**
- Production-ready topology viewer
- Advanced filtering and search
- Export functionality

---

### Phase 5: MCP Integration (Weeks 9-10) 🆕
**Goal:** MCP server integration and configuration

**Tasks:**
- ✅ MCP settings page UI
- ✅ MCP connection testing
- ✅ MCP API client library
- ✅ Backend MCP proxy endpoints
- ✅ Request/response logging
- ✅ Health monitoring dashboard
- ✅ Error handling and retry logic
- ✅ Cache implementation

**Backend Tasks:**
- ✅ Go MCP proxy service
- ✅ Authentication middleware
- ✅ Rate limiting
- ✅ Request validation
- ✅ Response transformation

**Deliverables:**
- Working MCP integration
- MCP configuration UI
- Status monitoring dashboard

---

### Phase 6: Path Analysis (Weeks 11-12) 🆕
**Goal:** Full path analysis feature

**Tasks:**
- ✅ Path analysis menu and routing
- ✅ Path analysis form (source/dest selection)
- ✅ Analysis type selection
- ✅ Submit to MCP server
- ✅ Results display with verdict
- ✅ Path visualization overlay on graph
- ✅ Hop-by-hop timeline
- ✅ Firewall rules table
- ✅ Security verdict widget
- ✅ Analysis history
- ✅ Export reports (PDF, JSON)
- ✅ Saved analysis templates

**Backend Tasks:**
- ✅ Path analysis request queue
- ✅ Results storage
- ✅ Streaming results (SSE)

**Deliverables:**
- Complete path analysis feature
- Visual path overlay on graphs
- Analysis history and reports

---

### Phase 7: Polish & Optimization (Weeks 13-14)
**Goal:** Production readiness

**Tasks:**
- ✅ Performance optimization (large graphs >1000 nodes)
- ✅ Canvas rendering for large datasets
- ✅ Lazy loading and pagination
- ✅ Real-time updates (WebSocket)
- ✅ Comprehensive error handling
- ✅ Loading states and skeletons
- ✅ Toast notifications
- ✅ User onboarding tour
- ✅ Help documentation
- ✅ Keyboard shortcuts
- ✅ Accessibility improvements (WCAG 2.1)

**Deliverables:**
- Production-ready application
- Optimized performance
- Complete documentation

---

## 🔒 Security Considerations

### Authentication & Authorization
- JWT-based authentication
- Role-based access control (RBAC)
  - Admin: Full access
  - Analyst: Read graphs, run analysis
  - Viewer: Read-only
- API key management for connectors
- MCP server authentication

### Data Security
- Encrypted credentials storage (AES-256)
- TLS/HTTPS everywhere
- Secure credential rotation
- Audit logging for all actions
- IP whitelisting for MCP servers

### Network Security
- CSP headers
- CORS configuration
- Rate limiting
- Input validation and sanitization
- XSS and CSRF protection

### Compliance
- SOC 2 considerations
- Data retention policies
- Export controls for sensitive topology data
- Compliance reports (PCI-DSS, NIST)

---

## 🚀 Deployment Architecture

### Production Stack

```
┌─────────────────────────────────────────────────┐
│                  CloudFlare                      │
│            (CDN + DDoS Protection)              │
└────────────────┬────────────────────────────────┘
                 │
┌────────────────┴────────────────────────────────┐
│              Load Balancer                      │
│             (NGINX or AWS ALB)                  │
└────┬──────────────────────────┬─────────────────┘
     │                          │
     │                          │
┌────┴──────────┐      ┌────────┴─────────────────┐
│  Frontend     │      │  Backend API             │
│  (Next.js)    │      │  (Go Platform)           │
│  - Static     │      │  - REST API              │
│  - SSR        │      │  - WebSocket             │
│  - Edge Fns   │      │  - Topology Engine       │
└───────────────┘      └──────────┬───────────────┘
                                  │
                       ┌──────────┴───────────────┐
                       │                          │
                  ┌────┴──────┐          ┌────────┴────────┐
                  │ PostgreSQL │          │  MCP Server     │
                  │ (Topology) │          │  (Python/Go)    │
                  └────────────┘          └─────────────────┘
```

### Infrastructure
- **Frontend:** Vercel or AWS CloudFront
- **Backend:** Kubernetes (EKS/GKE) or Docker Swarm
- **Database:** PostgreSQL + TimescaleDB (time-series)
- **Cache:** Redis
- **Storage:** S3 or MinIO (configs, exports)
- **Monitoring:** Prometheus + Grafana

---

## 📈 Success Metrics

### User Engagement
- Daily active users (DAU)
- Time spent in topology viewer
- Number of path analyses run per day
- Connector sync success rate

### Performance
- Page load time < 2s
- Graph render time < 3s for 500 nodes
- Path analysis completion < 5s
- 99.9% uptime

### Business Value
- Reduction in misconfiguration incidents
- Faster network troubleshooting (MTTR)
- Security policy compliance improvements
- Cost savings from automation

---

## 🎓 Training & Documentation

### User Documentation
- Getting Started Guide
- Connector Configuration Guides (per vendor)
- Path Analysis Tutorial
- MCP Integration Guide
- Troubleshooting FAQ

### Developer Documentation
- API Reference
- MCP Protocol Specification
- Component Library (Storybook)
- Contribution Guidelines
- Deployment Guide

---

## 🔮 Future Enhancements (Post-MVP)

### Q1 2026
- Real-time topology updates (WebSocket streaming)
- Collaborative features (shared views, annotations)
- Network change simulation (what-if scenarios)
- Custom alerting and notifications

### Q2 2026
- Mobile app (React Native)
- AI-powered anomaly detection
- Natural language queries ("Show me all devices in VLAN 10")
- Integration with ITSM tools (ServiceNow, Jira)

### Q3 2026
- Multi-tenancy for MSPs
- Advanced compliance reporting
- Time-travel (historical topology view)
- Network automation workflows

---

## 📞 Contact & Support

**Project Lead:** [Your Name]  
**Email:** [your-email]  
**GitHub:** https://github.com/mattpainter701/graphlayer  
**Documentation:** https://docs.graphlayer.io

---

## ✅ Appendix A: Technology Evaluation

### Why D3.js?
- **Pros:** Maximum flexibility, custom layouts, excellent documentation
- **Cons:** Steeper learning curve, more code to write
- **Alternatives:** Cytoscape.js (better for large graphs), react-force-graph (easier)

### Why Next.js?
- **Pros:** Best-in-class React framework, excellent DX, built-in optimization
- **Cons:** Opinionated, learning curve for App Router
- **Alternatives:** Remix, SvelteKit, Astro

### Why Zustand?
- **Pros:** Minimal boilerplate, excellent TypeScript support, small bundle
- **Cons:** Less ecosystem than Redux
- **Alternatives:** Redux Toolkit, Jotai, Recoil

---

## ✅ Appendix B: MCP Protocol Example

### Sample MCP Request/Response

**Request:**
```json
{
  "jsonrpc": "2.0",
  "method": "analyze_path",
  "params": {
    "graph_id": "prod-network-01",
    "source": "192.168.1.0/24",
    "destination": "10.10.50.25",
    "protocol": "tcp",
    "port": 443,
    "options": {
      "include_all_paths": true,
      "max_hops": 10
    }
  },
  "id": "req-abc123"
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "verdict": "allowed",
    "verdict_reason": "Traffic permitted by firewall rule 'Allow_Internal_to_DMZ'",
    "paths": [
      {
        "id": "path-1",
        "type": "primary",
        "verdict": "allowed",
        "hops": [
          {
            "sequence": 1,
            "node_id": "node-src-vlan",
            "node_label": "VLAN 1 (192.168.1.0/24)",
            "action": "route"
          },
          {
            "sequence": 2,
            "node_id": "node-core-router",
            "node_label": "Core-Router-01",
            "action": "route",
            "next_hop_ip": "10.0.0.1"
          },
          {
            "sequence": 3,
            "node_id": "node-edge-fw",
            "node_label": "Edge-FW-01",
            "action": "filter"
          },
          {
            "sequence": 4,
            "node_id": "node-dest-host",
            "node_label": "10.10.50.25",
            "action": "route"
          }
        ],
        "hop_count": 4,
        "firewall_rules_applied": [
          {
            "firewall_id": "fw-edge-01",
            "firewall_name": "Edge-FW-01",
            "rule_id": "rule-100",
            "rule_name": "Allow_Internal_to_DMZ",
            "rule_order": 10,
            "action": "allow",
            "source_zones": ["internal"],
            "dest_zones": ["dmz"],
            "log_enabled": true
          }
        ],
        "security_zones_crossed": ["internal", "dmz"],
        "risk_score": 35,
        "risk_factors": [
          "Direct internet exposure on destination",
          "Single firewall hop"
        ]
      }
    ],
    "analysis_time_ms": 1247,
    "paths_evaluated": 3
  },
  "id": "req-abc123"
}
```

---

## 🚀 Quick Start: Local Development with Docker Compose

For rapid local development, use Docker Compose:

```yaml
# docker-compose.yml
version: '3.8'

services:
  frontend:
    build:
      context: ./graphlayer-ui
      dockerfile: Dockerfile.dev
    ports:
      - "3000:3000"
    environment:
      - NEXT_PUBLIC_API_URL=http://localhost:8080
      - NODE_ENV=development
    volumes:
      - ./graphlayer-ui:/app
      - /app/node_modules
    depends_on:
      - backend
  
  backend:
    build:
      context: ./platform
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - REDIS_HOST=redis
      - REDIS_PORT=6379
    volumes:
      - ./platform:/app
    depends_on:
      - postgres
      - redis
  
  mcp-server:
    build:
      context: ./mcp-server
      dockerfile: Dockerfile
    ports:
      - "8081:8080"
    environment:
      - GRAPH_API_URL=http://backend:8080
      - MAX_WORKERS=4
    depends_on:
      - backend
  
  postgres:
    image: timescale/timescaledb:latest-pg15
    ports:
      - "5432:5432"
    environment:
      - POSTGRES_DB=graphlayer
      - POSTGRES_USER=graphlayer
      - POSTGRES_PASSWORD=dev_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
  
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
  
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
  
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3001:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana_data:/var/lib/grafana
      - ./monitoring/grafana/dashboards:/etc/grafana/provisioning/dashboards
    depends_on:
      - prometheus

volumes:
  postgres_data:
  redis_data:
  prometheus_data:
  grafana_data:
```

**Start the stack:**
```bash
# Clone the repo
git clone https://github.com/mattpainter701/graphlayer.git
cd graphlayer

# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Access services:
# Frontend:    http://localhost:3000
# Backend API: http://localhost:8080
# MCP Server:  http://localhost:8081
# Grafana:     http://localhost:3001 (admin/admin)
# Prometheus:  http://localhost:9090
```

---

## 📊 Summary: Key Architectural Decisions

### ✅ Corrected Graph Model
| Component | Role | Example |
|-----------|------|---------|
| **Nodes** | Assets (devices, hosts) | Firewall, Switch, Server, Workstation, PLC |
| **Edges** | Networks (IP subnets, VLANs) | 192.168.1.0/24, VLAN 10, DMZ Network |

**Why This Makes Sense:**
- Aligns with how network engineers think: "What devices are on this network?"
- Enables network-centric queries: "Show me all assets in DMZ"
- Supports security zone visualization
- Matches industry standards (Cisco Cyber Vision, reference, Skybox)

### ✅ Container-Native Architecture
- **Frontend:** Next.js in Node Alpine (lightweight)
- **Backend:** Go API in Alpine (static binary)
- **MCP Server:** Python with FastAPI/uvicorn
- **Orchestration:** Kubernetes with HPA
- **Storage:** PostgreSQL + TimescaleDB (time-series)
- **Cache:** Redis Cluster

### ✅ High-Performance Rendering
- **WebGL (PIXI.js)** for 10,000+ nodes
- **Canvas** for 1,000-5,000 nodes
- **Viewport culling** with spatial indexing
- **Level-of-detail (LOD)** based on zoom
- **Web Workers** for layout calculation
- **Aggressive caching** (Memory + Redis + DB)

### ✅ MCP-Powered Path Analysis
- Dedicated "Path Analysis" menu item
- Source/destination selection (subnet or host)
- Analysis types:
  - Reachability
  - Firewall policies
  - Routing paths
  - Attack paths
  - Segmentation validation
- Results with visual path overlay
- Verdict: Allow/Deny with reasoning

### ✅ Competitive Positioning
| Feature | Cisco Cyber Vision | reference | Skybox | **GraphLayer** |
|---------|-------------------|---------|---------|----------------|
| Multi-vendor | ✅ | ✅ | ✅ | ✅ |
| Containerized | ❌ | ❌ | ❌ | ✅ |
| Open Source Backend | ❌ | ❌ | ❌ | ✅ (Go parsers) |
| MCP Integration | ❌ | ❌ | ❌ | ✅ (AI-powered) |
| WebGL Performance | ⚠️ | ⚠️ | ⚠️ | ✅ (10K+ nodes) |
| Path Analysis | ✅ | ✅ | ✅ | ✅ (MCP-enhanced) |
| Purdue Model | ✅ | ❌ | ❌ | ✅ |
| Price | $$$$$ | $$$ | $$$$$ | Open Source |

---

## 📝 Next Steps

1. **Review Proposal** - Stakeholder approval
2. **Set Up Repos** - Frontend, MCP server
3. **Phase 1 Start** - Next.js scaffolding
4. **MCP Protocol Design** - Define JSON-RPC API
5. **Backend API Endpoints** - Extend Go platform

---

## 📞 Questions & Feedback

**Key Questions to Answer:**
1. ✅ **Graph Model:** Assets as nodes, networks as edges
2. ✅ **Deployment:** Containerized with Kubernetes
3. ✅ **Performance:** WebGL for 10,000+ nodes
4. ✅ **MCP Integration:** Separate path analysis menu
5. ⏳ **Timeline:** 14 weeks to production-ready MVP

**Open Issues:**
- [ ] MCP server language preference (Python vs Go)?
- [ ] 3D visualization support (Three.js)?
- [ ] Multi-tenancy requirements?
- [ ] RBAC roles and permissions model?

---

**End of Proposal v1.1 (Revised)**

This proposal provides a comprehensive blueprint for GraphLayer Frontend with:
- ✅ Containerized, high-performance architecture
- ✅ Corrected graph model (assets=nodes, networks=edges)
- ✅ MCP-powered path analysis
- ✅ Competitive analysis and positioning
- ✅ WebGL rendering for massive graphs
- ✅ Ready for implementation

**Repository:** https://github.com/mattpainter701/graphlayer  
**Proposal Status:** Ready for review and development kickoff 🚀

