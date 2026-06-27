# Agent Runtime Governance (ARG) Research & Loopers Alignment

Loopers' current position—a fast, open-source, fail-closed kill-switch for AI costs—is a powerful wedge into the emerging **Agent Runtime Governance (ARG)** market. However, as agent capabilities evolve, budget enforcement alone is insufficient. To become the dominant platform, Loopers must define and capture the standard for ARG.

This research document consolidates the latest industry data on ARG architectures, defining its core layers and aligning them with the Loopers Master Strategy.

---

## 1. What is Agent Runtime Governance (ARG)?

Traditional Identity and Access Management (IAM) and Security Information and Event Management (SIEM) systems operate on human timescales and lack "agent intent" context. **Agent Runtime Governance (ARG)** fills this gap. It is a control loop that sits between infrastructure identity and autonomous execution, providing real-time, runtime-native enforcement of agent behaviors.

### The Four Core Layers of ARG

While Observability and Enforcement are the two primary conceptual pillars, modern enterprise frameworks break ARG down into four distinct architectural layers:

#### Layer 1: Policy & Control Plane (The "Brain")
This layer manages *who* the agent is and *what* rules govern it.
*   **Cryptographic Identity:** Issuing verifiable identities (like SPIFFE/X.509) to individual agents (Non-Human Identities) so they can be authenticated.
*   **Policy Engine:** Evaluating actions against business logic in real time using frameworks like OPA/Rego or AWS Cedar.

#### Layer 2: Execution & Interception (The "Muscle")
This is where the actual blocking or allowing happens, sitting between the agent and its tools.
*   **Runtime Interceptor / AI Firewall:** An independent proxy that inspects every tool call, API request, or database query *before* it reaches the destination.
*   **Execution Boundaries:** Defining strict sandbox environments so compromised agents cannot break out of their designated domain.

#### Layer 3: Observability & Traceability (The "Memory")
Going far beyond standard logging to provide "decision provenance."
*   **Contextual Tracing & Semantic Logs:** Capturing the agent's goal, its reasoning, the policy evaluated, and the exact tool invocation chain.
*   **Forensic Replay & Drift Detection:** Maintaining an immutable audit trail to detect behavioral shifts (e.g., prompt injection) and allow auditors to reconstruct incidents.

#### Layer 4: Operational & Resource Management (The "Governor")
Protecting the underlying infrastructure and managing budgets.
*   **FinOps & Cost Governance:** Enforcing token budgets and hard financial caps per session or per agent.
*   **Blast-Radius Bounds & Circuit Breakers:** Implementing deterministic limits (e.g., max API calls per minute) and kill switches to halt runaway agents immediately.

---

## 2. Advanced ARG Frameworks

As the market matures, several frameworks are defining the "Gold Standard" for enterprise agent deployments.

### The Enforceability Ladder
Industry frameworks categorize governance maturity based on where and how enforcement occurs:
1.  **Aspirational/Documentary:** Policies exist on paper but have no runtime enforcement.
2.  **Attested:** Point-in-time claims about compliance.
3.  **Application-plane Enforced:** Policy engines deployed *inside* the agent's code. This is vulnerable because a compromised agent can bypass its own middleware.
4.  **Two-Plane Verified:** **The "Gold Standard."** Governance is enforced by an independent infrastructure layer (a separate control plane) outside the agent's process. It actively evaluates, authorizes, or vetoes every action, providing indisputable evidence of compliance.

### The Agent Bill of Materials (AgBOM)
Because agents are dynamic (discovering new tools or swapping MCP servers on the fly), traditional SBOMs are insufficient. The **AgBOM** dynamically inventories the agent's environment, tools, models, and evolving capabilities in real-time. This provides the supply-chain transparency required by emerging regulations like the EU AI Act.

---

## 3. Aligning ARG Research with the Loopers Master Strategy

To dominate the ARG market, Loopers must map its engineering roadmap directly to these industry layers, building from the ground up.

### Phase 1: Fortify the Core (Observability & Baseline Enforcement)
**Goal:** Build the "Blast Radius" controls and foundational Identity Plane.
*   **Expand the Wedge:** Move beyond financial budgets to implement **Blast-Radius Bounds** (Layer 2). Add deterministic rate limits on API frequencies, tool invocations, and token consumption per session. 
*   **Establish the Identity Plane:** Build an Agent Registry that issues cryptographic Non-Human Identities (NHI). Without identity, RBAC and trajectory sandboxing are impossible.
*   **Contextual Tracing:** Enhance the existing Loopers proxy to log not just costs, but "decision provenance" compatible with enterprise SIEMs (Layer 1).

### Phase 2: Expand the Control Plane (Trajectory & Ecosystem)
**Goal:** Implement Policy-as-Code and advanced drift detection.
*   **Trajectory Sandboxing:** Evolve from blocking atomic actions to governing the sequence of actions, ensuring an agent cannot "reason around" guardrails.
*   **Declarative Policy Language (DPL):** Introduce GitOps-friendly policy management for security teams to define rules outside the agent codebase.
*   **Dynamic AgBOM Generation:** Automatically generate an AgBOM for every registered agent, integrating deeply with the MCP ecosystem to track dynamic tool discovery.

### Phase 3: Define the Category (Two-Plane Verified Leadership)
**Goal:** Become the undeniable standard for "Two-Plane Verified" architecture.
*   **Architectural Separation:** Formalize the split between the Loopers OSS Proxy (the muscle/enforcement) and the Loopers SaaS Control Plane (the brain/policy).
*   **Cryptographic Proof of Enforcement:** Generate tamper-evident receipts for every allowed/denied tool call, proving to auditors that policies were enforced from the outside.
*   **"Loopers Verified" Ecosystem:** Launch a certification program for frameworks and enterprises, establishing Loopers as the universal, anti-lock-in governance layer for multi-vendor agent fleets.

### The Core Message for Loopers
> *"You will run agents from Salesforce, Microsoft, and your own homegrown stack. They will all be governed by one independent, unbypassable, and auditable control plane utilizing Two-Plane Verified architecture. That control plane is Loopers."*