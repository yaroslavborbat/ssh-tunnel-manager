# SSH Tunnel Manager

A lightweight Go service to manage SSH tunnels and ensure they stay active.

## ✨ Features

- Simple YAML-based configuration for defining tunnels.
- Monitors and maintains SSH tunnels to ensure they're always up.

---

## ⚙️ Configuration

The configuration is defined in a YAML file. A sample configuration can be found in the file [example-config.yaml](./example-config.yaml)

### Configuration Fields

#### Top-Level Fields

- **defaultUser**: Default username for SSH connections (optional if specified per tunnel).
- **defaultBindIP**: Default bind address for local forwarding (e.g., `127.0.0.1`).
- **defaultPrivateKeyPath**: Path to the default private key for SSH authentication.
- **defaultPassPhrasePath**: Path to the file containing the passphrase for the private key.

#### Tunnel-Specific Fields

Each tunnel can override the default configuration:

- **name**: Unique identifier for the tunnel.
- **user**: (Optional) Username for SSH login. If not provided, the `defaultUser` is used.
- **host**: Remote server (hostname or IP address) to connect to via SSH.
- **hostIP**: Address of the target service on the remote machine. This is the internal IP or hostname on the remote side that the traffic will be forwarded to.
- **hostPort**: Port of the target service on the remote machine.
- **bindIP**: (Optional) Local IP address on the client (your machine) where the tunnel will listen. Defaults to `defaultBindIP`. Common values:
    - `127.0.0.1`: Makes the port accessible only locally.
    - `0.0.0.0`: Makes the port accessible from all network interfaces.
- **bindPort**: Local port on the client where the tunnel will listen.
- **privateKeyPath**: (Optional) Path to the private SSH key used for this tunnel. If not specified, the `defaultPrivateKeyPath` is used.
- **passPhrasePath**: (Optional) Path to the file containing the passphrase for the private key. If not specified, the `defaultPassPhrasePath` is used.

## 🚀 Installation

To set up the SSH Tunnel Manager, follow these steps:

### Building the Docker Image

To build the Docker image, use the following command:
```bash
    task docker:build
```

This will create a Docker image with the SSH Tunnel Manager pre-configured for use.

### Deployment with Systemd and Docker Compose

1. Customize the files:
    - [`docker-compose.yaml`](./docker-compose.yaml)
    - [`ssh-tunnel-manager.service`](./ssh-tunnel-manager.service)

2. Copy the customized `systemd` unit file to your system's `systemd` directory, reload `systemd`, and start the service:
    ```bash
    sudo cp ssh-tunnel-manager.service /etc/systemd/system/
    sudo systemctl daemon-reload
    sudo systemctl enable ssh-tunnel-manager
    sudo systemctl start ssh-tunnel-manager 
   ```
---

This installation method leverages `docker-compose` and `systemd` for a reliable, integrated deployment.