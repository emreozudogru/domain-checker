# Domain Expiry Monitor

An ultra-lightweight, high-performance Domain Expiry Monitor built in Go, designed specifically for minimal resource usage on ARM64 architectures (like the Raspberry Pi 5). 

## Highlights & Features
- **Zero Third-Party Dependencies:** Uses standard library only (TCP WHOIS sockets, Native JS/HTTP). Zero bloat.
- **Minimal Footprint:** Compiles to a single static binary running inside a Linux `scratch` container.
- **Specific TLD Parsing:** Custom WHOIS parser mapping designed exactly for `.com`, `.net`, `.org`, `.us`, and `.tr` extensions natively.
- **File-based Configuration (Single Source of Truth):** No DB overhead; domains are strictly read from a highly-portable `domains.json` file.
- **Dark Mode UI:** A clean, readable, single-file HTML/CSS UI fully embedded into the static binary at compile-time. Avoids any frontend frameworks.
- **Alerting Ready:** Foundational Go STMP structure prepared for optional domain drop alerts using zero resources unless enabled (`enable_email_alerts: true`).

## Setup & configuration
Edit or mount `domains.json` to define your domain watch-list:
```json
{
  "domains": [
    "example.com",
    "nic.tr",
    "golang.org"
  ],
  "enable_email_alerts": false,
  "smtp_config": {
    "host": "smtp.example.com",
    "port": 587,
    "user": "alert@example.com",
    "pass": "secret",
    "to": "admin@example.com"
  }
}
```

## Arcane Docker UI Instructions (CRITICAL)

Follow these exact steps to deploy this repository flawlessly onto your Raspberry Pi 5 via Arcane Docker UI:

### 1. Connecting and Pulling the Repository
- Open your **Arcane Docker UI** dashboard.
- Navigate to the **Apps** or **Stacks** view section depending on your Arcane version.
- Select **Deploy from Repository** or **Add New Git Repository**.
- Input the **URL of this Git repository** and provide credentials if it's a private repository. 
- Wait for Arcane to index the files and locate the provided `docker-compose.yml` file gracefully.

### 2. Configuring the Deployment / Stack
- Select the option to **Deploy via Compose**. Arcane natively respects the lightweight settings predefined in `docker-compose.yml`.
- (Optional) Toggle **Enable Automatic Updates** if you plan to update the `domains.json` list directly via remote Git commits.
- Set the **Stack Name** to `domain-monitor` (or anything intuitive).
- Review the predefined environment. Port `8080` is automatically exposed to your host system allowing UI access.

### 3. Mounting the external `domains.json` File Natively in Arcane
To ensure the UI remains strictly read-only and uses minimal resources, domains are configured externally using Arcane's volume features:
1. Inside the Arcane compose deployment configuration, locate the **Volumes** or **Mounts** list for the `domain-monitor` service.
2. The template currently states `./domains.json:/app/domains.json:ro`. 
3. **If you are bypassing the git copy of domains.json:** Create a file directly on your Raspberry Pi host OS (e.g., `/home/pi/domains.json`).
4. In Arcane UI's Mounts Interface, map the **Host Path** (e.g., `/home/pi/domains.json`) directly to the **Container Path** `/app/domains.json`.
5. Ensure the Mode toggle is explicitly set to **Read-Only (ro)** to prevent any accidental overwrites.
6. Click **Deploy Stack**. The builder script inside `Dockerfile` will automatically compile an ARM64 binary and discard all compilation tools, leaving an empty `scratch` environment for runtime.

## Open-Source Tech Stack & Licensing Requirements

- **Backend:** Golang 1.22 (`BSD-3-Clause`) Standard library packages only. No 3rd-party dependencies.
- **Frontend Assets:** Embedded HTML Template. (No frameworks, Custom Vanilla HTML/CSS/JS script built ground-up).
- **Execution Environment:** Docker Container utilizing `golang:1.22-alpine` builder and `scratch` deployment base.
- **Project License:** Open-source MIT License.
