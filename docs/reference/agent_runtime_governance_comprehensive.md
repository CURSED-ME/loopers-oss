# Agent Runtime Governance (ARG): Comprehensive Implementation Study

> [!IMPORTANT]
> This document serves as an exhaustive, granular research study into the architectural implementations of Agent Runtime Governance (ARG). Derived from industry standards, academic research, and emerging frameworks, it details how to achieve "Two-Plane Verified" security for autonomous AI agents. This document provides the technical foundation for building the Loopers platform.

---

## 1. Introduction: The Enforceability Ladder

Agent security cannot rely on prompt engineering or probabilistic guardrails. The industry standard for evaluating agent governance maturity is the **Enforceability Ladder**:

| Rung | Level | Description | Risk Profile |
| :--- | :--- | :--- | :--- |
| **1** | Aspirational | Stated ethical guidelines with no technical enforcement. | Extreme |
| **2** | Documentary | Written policies and rules. | High |
| **3** | Attested | Point-in-time configuration checks. | Medium |
| **4** | Application-plane Enforced | Guardrails built *inside* the agent's code. | Vulnerable to bypass |
| **5** | **Two-Plane Verified** | Governance enforced by an independent infrastructure layer. | **The Gold Standard** |

> [!NOTE]
> To achieve Rung 5, ARG is broken into four distinct architectural layers, detailed below.

---

## 2. Layer 1: Policy & Control Plane (The "Brain")

This layer handles identity issuance and deterministic policy evaluation. It separates *who* the agent is from *what* it is allowed to do.

### 2.1. Non-Human Identity (NHI) via SPIFFE/SPIRE
Static API keys are insufficient for dynamic agents. The industry standard for workload identity is **SPIFFE** (Secure Production Identity Framework for Everyone).

* **Architecture:** Agents do not possess long-lived secrets. Instead, a **SPIRE Server** issues highly ephemeral, cryptographically verifiable identities known as **SVIDs** (SPIFFE Verifiable Identity Documents, usually X.509 certificates or JWTs).
* **Attestation:** Before receiving an SVID, the **SPIRE Agent** physically verifies the agent process's environment (e.g., its Kubernetes namespace, container hash, or cloud metadata).
* **Decoupling Identity:** 
  * The SVID proves the agent's *machine identity*. 
  * If the agent acts on behalf of a human, a separate OAuth/OIDC token is passed alongside the SVID to prove *delegated authority*.

### 2.2. Deterministic Policy Engines
Instead of asking an LLM to "follow rules," runtime governance intercepts actions and asks a deterministic policy engine if the action is permitted. 

| Feature | Open Policy Agent (OPA) | AWS Cedar |
| :--- | :--- | :--- |
| **Language** | Rego (Declarative, Datalog-based) | Cedar (Purpose-built for auth) |
| **Mechanism** | The proxy sends a JSON request containing the agent's identity and tool call. OPA evaluates against in-memory Rego policies. | Natively integrated into Amazon Verified Permissions. Policies follow a strict `Principal -> Action -> Resource` structure. |
| **Advantage** | Cloud-native standard, highly flexible, framework agnostic. | Mathematically verifiable policies and extremely low latency. |

---

## 3. Layer 2: Execution & Interception (The "Muscle")

This layer intercepts the agent's intent before it impacts the real world. 

### 3.1. Out-of-Process Interception
To achieve Two-Plane Verification, interception must occur outside the agent process.

* **The AI Firewall/Proxy:** A reverse proxy or API gateway (e.g., Kong AI Gateway, specialized MCP proxies) sits at the network boundary.
* **Protocol Mediation:** As agents increasingly use the **Model Context Protocol (MCP)**, the proxy acts as an intermediary, parsing incoming **JSON-RPC 2.0** messages. The proxy strips the arguments from a tool-call request, formats them, and queries the Layer 1 Policy Engine.

### 3.2. Execution Sandboxing
When a tool call *is* permitted, the execution environment must still be physically constrained to prevent zero-day exploits or malicious payloads.

* **OS-Level Primitives:** Using **seccomp** (Secure Computing Mode) or **eBPF** (Extended Berkeley Packet Filter) to block unauthorized system calls.
* **MicroVMs:** Isolating tool execution entirely using technologies like AWS Firecracker or gVisor, ensuring that even a successful escape is contained to an ephemeral, stateless environment.

---

## 4. Layer 3: Observability & Traceability (The "Memory")

> [!TIP]
> ARG requires "decision provenance"—the ability to reconstruct exactly why an agent took an action.

### 4.1. OpenTelemetry (OTel) GenAI Semantic Conventions
Standard application monitoring fails to capture agent intent. The industry is adopting **OpenTelemetry GenAI Semantic Conventions** (`gen_ai.*`).

* **Span Hierarchies:** Agent execution is modeled as a tree. The root span is the user request; child spans represent reasoning steps, retrieval operations (RAG), and individual tool calls.
* **Semantic Attributes:** Traces are enriched with specific attributes like `agent.decision`, `gen_ai.prompt`, `gen_ai.completion`, and `agent.tool.name`.
* **Evidence Attachment:** Contextual evidence (the specific policy evaluated, the SPIFFE ID, the retrieved context) is attached as span events. This allows auditors to visualize the exact chain of logic without guessing.

### 4.2. Drift Detection
By maintaining a structured, OTel-compliant semantic log, ARG platforms can run algorithmic drift detection. 
* *Example:* Establishing baselines of standard tool usage (e.g., *Agent A usually queries the HR database 5 times a day*). The system can automatically trigger alerts or human-in-the-loop (HITL) workflows if the agent suddenly attempts to query a financial database.

---

## 5. Layer 4: Operational & Resource Management (The "Governor")

This layer protects the underlying infrastructure and financial health of the deployment.

### 5.1. Deterministic Circuit Breakers
Unlike passive logging, circuit breakers actively halt execution based on operational heuristics.
* **Frequency Limiting:** Hard caps on API calls per minute to prevent runaway loops (e.g., an agent repeatedly failing to read a file and trying again in an infinite loop).
* **Blast-Radius Bounds:** Restricting the maximum number of external systems an agent can touch in a single session.

### 5.2. FinOps & Token Budgets
Financial constraints applied at the proxy layer. Intercepting requests to cloud LLM providers, calculating token cost estimates, and blocking requests that exceed predefined session, user, or agent-level budgets.

---

## 6. Advanced Compliance & Verification

For highly regulated industries, ARG provides verifiable artifacts of compliance.

### 6.1. The Agent Bill of Materials (AgBOM)
Traditional static SBOMs cannot capture self-modifying agents. The AgBOM dynamically extends standards like **CycloneDX** or **SPDX**.

* **Dynamic Inventory:** As an agent connects to new MCP servers or loads new models into context, the AgBOM updates in real time.
* **Supply Chain Transparency:** It provides a machine-readable artifact of exactly what tools, libraries, and models comprise the agent at any given millisecond, fulfilling requirements for standards like the EU AI Act.

### 6.2. Cryptographic Proof of Enforcement (Action Receipts)
To prevent "AI-washing," the control plane generates a cryptographically signed receipt (using hash-linking or Merkle proofs) for every allowed or denied tool execution. 

> [!WARNING]
> This receipt proves to an offline auditor that:
> 1. The tool was called.
> 2. The independent policy engine evaluated it.
> 3. The exact state of the policy at that timestamp allowed it. 

---

## 7. Loopers Architecture Alignment

To build the dominant ARG platform, Loopers will map its development directly to these implementations:

* **Layer 2 & 4 (Current Strength):** Loopers' core strength is currently its out-of-process, sub-millisecond proxy intercepting traffic and enforcing FinOps budgets. This must be immediately expanded to include Blast-Radius circuit breakers.
* **Layer 1 (Immediate Next Step):** Implement an Agent Registry issuing SPIFFE-compatible identities, coupled with an OPA or Cedar integration for deterministic policy evaluation.
* **Layer 3 (Parallel Step):** Adopt OpenTelemetry GenAI conventions to export "decision provenance" traces.
* **Advanced (Future Moat):** Introduce AgBOM generation and Cryptographic Action Receipts as Enterprise SaaS features.
