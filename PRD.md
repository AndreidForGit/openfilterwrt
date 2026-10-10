# Product Requirements Document (PRD): OpenWrt Mobile Controller

## 1. Project Overview
The **OpenWrt Mobile Controller** is a lightweight backend service written in **Golang** paired with a mobile-optimized **Progressive Web App (PWA)**. It allows administrators to manage OpenWrt router settings (specifically MAC address whitelisting and static DHCP allocations) securely from mobile devices. The backend service is containerized using **Docker** for fast, repeatable deployments.

---

## 2. Engineering & Code Quality Rules
Any code generated or maintained for this project must strictly adhere to the following developer constraints:
* **No Single-Letter Variables:** Variable names must be fully descriptive words (e.g., use `routerConnection` instead of `c`).
* **No Abbreviated Variables:** Avoid shortening words (e.g., use explicit descriptive identifiers without truncation).
* **No One-Liners:** Avoid method chaining, functional composition, syntactic sugar expressions, inline anonymous functions, short-circuit evaluation, and destructuring/tuple packing. Every control flow statement, assignment, and function call must be broken down into explicit, multi-line blocks.
* **No Adding Dependencies:** Minimize external packages to the absolute minimum required by the core architecture.

---

## 3. Rule Exception Matrix

| Rule Violated | Justification | Alternatives Considered & Why Rejected |
| :--- | :--- | :--- |
| **No Adding Dependencies** | The Go standard library does not include a native SSH client protocol implementation. The official `golang.org/x/crypto/ssh` package is strictly required to establish secure communication sessions with the OpenWrt router. | *Alternative 1:* Using `os/exec` to invoke the host system `ssh` binary.<br>*Rejection Reason:* Introduces fragile text parsing of CLI outputs, depends on container binary availability, and lacks robust programmatic error handling. |

---

## 4. Technical Stack
* **Backend Language:** Golang (using standard library packages like `net/http`, `encoding/json`, `os`, `fmt`).
* **SSH Protocol:** Official Go crypto library (`golang.org/x/crypto/ssh`).
* **Frontend:** Mobile-optimized Progressive Web App (PWA) served statically with tailwind cdn.
* **Deployment & Containerization:** Docker (utilizing multi-stage builds to maintain a minimal image footprint).

---

## 5. Documentation Requirements (`README.md`)
A root-level `README.md` file must be maintained and updated immediately whenever a functional or structural change is introduced to the repository. It must contain:
1. **System Overview:** Brief architectural description.
2. **Prerequisites:** Requirements for local development and Docker execution.
3. **Configuration:** Environment variables required (e.g., router IP address, credentials).
4. **Deployment Instructions:** Step-by-step Docker build and run commands.