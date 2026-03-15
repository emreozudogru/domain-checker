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

## Arcane App (getarcane.app) Deployment Instructions (CRITICAL)

[Arcane](https://getarcane.app/) is a modern, lightweight Docker client and management UI. To run this project using Arcane, follow these steps:

### 1. Connecting to the Project
- Clone or download this Git repository to your local machine (or wherever your Arcane environment is running).
- Ensure the `domains.json` file is present in the root of the project folder alongside `docker-compose.yml`.

### 2. Configuring the Deployment
- Open **Arcane** and navigate to its environment dashboard.
- If Arcane supports direct project import, add the `domain-checker` folder as a new Compose project. 
- Alternatively, you can run `docker-compose up -d --build` in your terminal from the project folder, and Arcane will automatically detect and manage the `domain-monitor` stack in its UI.
- Port `8080` will be exposed to your local network, making the Domain Monitor dashboard accessible at `http://localhost:8080`.

### 3. Mounting the external `domains.json` File
This structure ensures the `domains.json` file acts as the single source of truth without being baked into the image.
1. The `docker-compose.yml` is already configured to mount `./domains.json` to `/app/domains.json:ro`. 
2. Because it uses a relative path (`./domains.json`), Arcane will successfully bind the file from your local repository folder directly into the container.
3. The `:ro` (read-only) flag ensures the container cannot accidentally overwrite or modify your local configuration file.
4. To add or remove domains, simply open the local `domains.json` file in any text editor, save your changes, and the background ticker will detect the new domains on its next cycle (or you can restart the container via the Arcane UI to force an immediate update).

## Open-Source Tech Stack & Licensing Requirements

- **Backend:** Golang 1.22 (`BSD-3-Clause`) Standard library packages only. No 3rd-party dependencies.
- **Frontend Assets:** Embedded HTML Template. (No frameworks, Custom Vanilla HTML/CSS/JS script built ground-up).
- **Execution Environment:** Docker Container utilizing `golang:1.22-alpine` builder and `scratch` deployment base.
- **Project License:** Open-source MIT License.
