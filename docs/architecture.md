# ArgoCD Addons Platform — Architecture

## System Overview

```mermaid
flowchart TB
    subgraph External["External Systems"]
        GIT["Git Repository<br/>(GitHub / Azure DevOps)"]
        ARGO["ArgoCD Server"]
        DD["Datadog API"]
        AI["AI Provider<br/>(Ollama / Claude / OpenAI / Gemini)"]
        HELM["Helm Chart Registries"]
    end

    subgraph Backend["Go Backend (aap-server)"]
        API["HTTP API Layer<br/>/api/v1/*"]
        SVC["Service Layer"]
        GITP["Git Provider"]
        ARGOC["ArgoCD Client"]
        DDC["Datadog Client"]
        AIC["AI Client + Agent"]
        HELMC["Helm Fetcher + Diff"]
        CFG["Config Parser<br/>(YAML)"]
    end

    subgraph Frontend["React Frontend (SPA)"]
        DASH["Dashboard"]
        CLUST["Clusters"]
        ADDON["Addon Catalog"]
        VMAT["Version Matrix"]
        OBS["Observability"]
        UPG["Upgrade Checker"]
        AICHAT["AI Assistant"]
        CONN["Connections"]
    end

    Frontend -->|fetch /api/v1/*| API
    API --> SVC
    SVC --> GITP --> GIT
    SVC --> ARGOC --> ARGO
    SVC --> DDC --> DD
    SVC --> AIC --> AI
    SVC --> HELMC --> HELM
    SVC --> CFG
```

## Data Flow: Git to UI

```mermaid
flowchart LR
    subgraph Sources["Sources of Truth"]
        GIT["Git Repo<br/>cluster-addons.yaml<br/>addons-catalog.yaml"]
        ARGO["ArgoCD<br/>Clusters + Apps<br/>Health + Sync"]
    end

    subgraph Processing["Backend Processing"]
        PARSE["YAML Parser"]
        ENRICH["Enrichment<br/>Merge Git config<br/>with ArgoCD state"]
        MODELS["Response Models<br/>Clusters, Addons,<br/>Health Stats"]
    end

    subgraph Views["Frontend Views"]
        UI["Dashboard / Clusters<br/>Addon Catalog<br/>Version Matrix<br/>Observability"]
    end

    GIT -->|GetFileContent| PARSE
    PARSE -->|Clusters + Addons| ENRICH
    ARGO -->|Health, Sync, Versions| ENRICH
    ENRICH --> MODELS
    MODELS -->|JSON API| UI
```

## Backend Package Architecture

```mermaid
flowchart TB
    subgraph cmd["cmd/server"]
        MAIN["main.go<br/>Entry point"]
    end

    subgraph api["internal/api"]
        ROUTER["router.go<br/>45+ endpoints"]
        HANDLERS["Handler files<br/>clusters, addons,<br/>dashboard, agent,<br/>datadog, upgrade,<br/>connections, ai_config"]
    end

    subgraph service["internal/service"]
        CLUSTSVC["ClusterService"]
        ADDONSVC["AddonService"]
        DASHSVC["DashboardService"]
        OBSSVC["ObservabilityService"]
        UPGSVC["UpgradeService"]
        CONNSVC["ConnectionService"]
    end

    subgraph providers["Integrations"]
        GITP["gitprovider<br/>GitHub / AzureDevOps"]
        ARGOC["argocd<br/>REST Client"]
        AIC["ai<br/>Client + Agent + Tools"]
        DDC["datadog<br/>Metrics Client"]
        HELMC["helm<br/>Fetcher + Diff"]
    end

    subgraph infra["Infrastructure"]
        CFG["config<br/>Parser + Store"]
        MODELS["models<br/>Shared types"]
        PLATFORM["platform<br/>K8s vs Local detect"]
    end

    MAIN --> ROUTER
    ROUTER --> HANDLERS
    HANDLERS --> service
    service --> providers
    service --> infra
    providers --> infra
```

## API Endpoint Map

```mermaid
flowchart LR
    subgraph Health
        H1["GET /health"]
    end

    subgraph Clusters
        C1["GET /clusters"]
        C2["GET /clusters/:name"]
        C3["GET /clusters/:name/comparison"]
        C4["GET /clusters/:name/values"]
        C5["GET /clusters/:name/config-diff"]
    end

    subgraph Addons
        A1["GET /addons/catalog"]
        A2["GET /addons/:name"]
        A3["GET /addons/:name/values"]
        A4["GET /addons/version-matrix"]
    end

    subgraph Dashboard
        D1["GET /dashboard/stats"]
        D2["GET /dashboard/pull-requests"]
    end

    subgraph Observability
        O1["GET /observability/overview"]
    end

    subgraph Upgrade
        U1["GET /upgrade/:addon/versions"]
        U2["POST /upgrade/check"]
        U3["POST /upgrade/ai-summary"]
        U4["GET /upgrade/ai-status"]
    end

    subgraph AI
        AI1["GET /ai/config"]
        AI2["POST /ai/provider"]
        AI3["POST /ai/test"]
        AI4["POST /agent/chat"]
        AI5["POST /agent/reset"]
    end

    subgraph Datadog
        DD1["GET /datadog/status"]
        DD2["GET /datadog/metrics/:ns"]
        DD3["GET /datadog/cluster-metrics/:name"]
    end

    subgraph Connections
        CN1["GET /connections/"]
        CN2["POST /connections/"]
        CN3["DELETE /connections/:name"]
        CN4["POST /connections/active"]
        CN5["POST /connections/test"]
    end
```

## Frontend Route Map

```mermaid
flowchart TB
    subgraph Layout["Layout (Sidebar + Content)"]
        NAV["Sidebar Navigation"]
    end

    NAV --> DASH["/ Dashboard<br/>Stats, health pie, PRs"]
    NAV --> CLUST["/ clusters<br/>Overview table + filters"]
    CLUST --> CDET["/clusters/:name<br/>Addon comparison, config diff"]
    NAV --> ACAT["/addons<br/>Catalog grid/list"]
    ACAT --> ADET["/addons/:name<br/>Per-cluster deployments"]
    NAV --> VMAT["/version-matrix<br/>Clusters × Addons table"]
    NAV --> OBS["/observability<br/>Health groups, sync timeline"]
    NAV --> UPG["/upgrade<br/>Helm diff + AI analysis"]
    NAV --> AICHAT["/ai-assistant<br/>Multi-turn chat"]
    NAV --> CONN["/connections<br/>Git + ArgoCD setup"]
```

## AI Agent Tool Calling Flow

```mermaid
sequenceDiagram
    participant U as User (Browser)
    participant FE as React Frontend
    participant API as Go API
    participant Agent as AI Agent
    participant LLM as LLM Provider
    participant Tools as Platform Tools

    U->>FE: Type question
    FE->>API: POST /agent/chat
    API->>Agent: Chat(sessionId, message)
    Agent->>LLM: Send prompt + tools
    LLM-->>Agent: Tool call (e.g., list_clusters)
    Agent->>Tools: Execute tool
    Tools-->>Agent: Tool result (JSON)
    Agent->>LLM: Send tool result
    Note over Agent,LLM: Repeat up to 5 iterations
    LLM-->>Agent: Final text response
    Agent-->>API: Response string
    API-->>FE: {session_id, response}
    FE->>FE: Typewriter effect
    FE-->>U: Streaming display
```

## Deployment Architecture

```mermaid
flowchart TB
    subgraph Build["Docker Multi-Stage Build"]
        S1["Stage 1: Node 22 Alpine<br/>npm install + vite build<br/>→ ui/dist/"]
        S2["Stage 2: Go 1.24 Alpine<br/>go build<br/>→ aap-server binary"]
        S3["Stage 3: Alpine 3.21<br/>aap-server + ui/dist<br/>Port 8080"]
        S1 --> S3
        S2 --> S3
    end

    subgraph K8s["Kubernetes (argocd-addons-platform namespace)"]
        CM["ConfigMap<br/>aap-server-config"]
        SEC["Secret<br/>aap-env-secrets"]
        DEP["Deployment<br/>1 replica, 128-512Mi"]
        SVC["Service<br/>ClusterIP:8080"]
        ING["Ingress / Port-Forward"]

        CM --> DEP
        SEC --> DEP
        DEP --> SVC --> ING
    end

    S3 -->|docker push| K8s
```

## External Integration Points

```
┌─────────────────────────────────────────────────────────────────┐
│                    AAP Backend (Go)                              │
│                                                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │ Git      │  │ ArgoCD   │  │ Datadog  │  │ AI Provider   │  │
│  │ Provider │  │ Client   │  │ Client   │  │ Client+Agent  │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────┬────────┘  │
│       │              │              │               │           │
└───────┼──────────────┼──────────────┼───────────────┼───────────┘
        │              │              │               │
        ▼              ▼              ▼               ▼
   ┌─────────┐  ┌───────────┐  ┌──────────┐  ┌─────────────┐
   │ GitHub  │  │ ArgoCD    │  │ Datadog  │  │ Ollama      │
   │ API     │  │ REST API  │  │ API      │  │ Claude      │
   │         │  │ (bearer)  │  │ (apikey) │  │ OpenAI      │
   │ Azure   │  │           │  │          │  │ Gemini      │
   │ DevOps  │  │           │  │          │  │             │
   └─────────┘  └───────────┘  └──────────┘  └─────────────┘
     OAuth2/       JWT           API+App       API Key /
     PAT           Token         Keys          Local (Ollama)

   Required ✓      Required ✓   Optional       Optional
   (core data)    (live state)  (metrics)      (AI features)
```
