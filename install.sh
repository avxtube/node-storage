#!/bin/bash

# Storage Node Installation Script
# Usage: curl -fsSL https://raw.githubusercontent.com/avxtube/node-storage/main/install.sh | sudo -E bash -s -- [OPTIONS]

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Defaults
UNINSTALL=false
DATABASE_URL=""
STORAGE_ID=""
STORAGE_ENCRYPTION_KEY=""
PORT="8888"
HOST="0.0.0.0"

APP_NAME="storage-node"
APP_DIR="/opt/$APP_NAME"
SERVICE_NAME="storage-node"
GITHUB_REPO="avxtube/node-storage"
RELEASES_URL="https://github.com/$GITHUB_REPO/releases/latest/download"

print_status()  { echo -e "${GREEN}[INFO]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error()   { echo -e "${RED}[ERROR]${NC} $1"; }

# Parse args
while [[ $# -gt 0 ]]; do
    case $1 in
        --uninstall)         UNINSTALL=true; shift ;;
        --database-url)      DATABASE_URL="$2"; shift 2 ;;
        --storage-id)        STORAGE_ID="$2"; shift 2 ;;
        --storage-encryption-key) STORAGE_ENCRYPTION_KEY="$2"; shift 2 ;;
        --port)              PORT="$2"; shift 2 ;;
        --host)              HOST="$2"; shift 2 ;;
        -h|--help)
            echo "Storage Node Installer"
            echo ""
            echo "Usage: curl -fsSL https://raw.githubusercontent.com/$GITHUB_REPO/main/install.sh | sudo -E bash -s -- [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --uninstall          Uninstall completely"
            echo "  --database-url URI   MongoDB connection string (DATABASE_URL)"
            echo "  --storage-id ID      Storage ID (REQUIRED — record ใน storages ของเครื่องนี้)"
            echo "  --storage-encryption-key KEY  Key used to decrypt S3 credentials"
            echo "  --port PORT          HTTP port (default: 8888)"
            echo "  --host HOST          HTTP bind address (default: 0.0.0.0)"
            echo "  -h, --help           Show this help"
            echo ""
            echo "Example:"
            echo "  curl -fsSL https://raw.githubusercontent.com/$GITHUB_REPO/main/install.sh | sudo -E bash -s -- \\"
            echo "      --database-url \"mongodb+srv://user:pass@host/db\" --storage-id \"storage-uuid\" --storage-encryption-key \"shared-secret\""
            exit 0 ;;
        *)
            print_error "Unknown option: $1"; exit 1 ;;
    esac
done

# ─── Uninstall ────────────────────────────────────────────────
if [ "$UNINSTALL" = true ]; then
    print_warning "⚠️  Starting Uninstallation..."
    systemctl stop "${SERVICE_NAME}"    2>/dev/null || true
    systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
    [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ] && rm "/etc/systemd/system/${SERVICE_NAME}.service"
    systemctl daemon-reload
    [ -d "$APP_DIR" ] && rm -rf "$APP_DIR"
    print_status "✅ Uninstalled successfully! (stored media is not removed)"
    exit 0
fi

# Check root
if [ "$(id -u)" -ne 0 ]; then
    print_error "This script must be run as root (use sudo)"
    exit 1
fi

if [ -z "$STORAGE_ID" ]; then
    print_error "--storage-id is required (binary จะ exit เองถ้าไม่ตั้ง)"
    exit 1
fi

print_status "🚀 Starting Installation..."

# ─── System Dependencies ──────────────────────────────────────
print_status "Installing system dependencies (curl)..."
if command -v apt-get &>/dev/null; then
    apt-get update -qq
    apt-get install -y -qq curl
elif command -v yum &>/dev/null; then
    yum install -y curl
elif command -v dnf &>/dev/null; then
    dnf install -y curl
fi

# ─── Stop existing service ────────────────────────────────────
systemctl stop ${SERVICE_NAME} 2>/dev/null || true

# ─── Create app directory ─────────────────────────────────────
print_status "Creating app directory: $APP_DIR"
mkdir -p "$APP_DIR"
cd "$APP_DIR"

# ─── Download binary ──────────────────────────────────────────
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
    BINARY="linux"
elif [ "$ARCH" = "aarch64" ]; then
    BINARY="linux-arm64"
else
    print_error "Unsupported architecture: $ARCH"
    exit 1
fi

print_status "Downloading binary ($BINARY) from latest release..."
curl -fsSL "$RELEASES_URL/$BINARY" -o "$APP_DIR/$APP_NAME"
chmod +x "$APP_DIR/$APP_NAME"

# ─── Create .env ─────────────────────────────────────────────
print_status "Creating .env file..."
cat > "$APP_DIR/.env" <<EOF
DATABASE_URL=$DATABASE_URL
STORAGE_ID=$STORAGE_ID
STORAGE_ENCRYPTION_KEY=$STORAGE_ENCRYPTION_KEY
PORT=$PORT
HOST=$HOST
EOF

# ─── Systemd service ──────────────────────────────────────────
print_status "Creating systemd service..."
cat > /etc/systemd/system/${SERVICE_NAME}.service <<EOF
[Unit]
Description=VdoHide Storage Node
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/$APP_NAME
Restart=always
RestartSec=5
EnvironmentFile=$APP_DIR/.env

[Install]
WantedBy=multi-user.target
EOF

# ─── Enable & start ───────────────────────────────────────────
systemctl daemon-reload
systemctl enable ${SERVICE_NAME}
systemctl start ${SERVICE_NAME}

sleep 2
echo ""
echo "============================================"
if systemctl is-active --quiet ${SERVICE_NAME}; then
    print_status "✅ Installation completed successfully!"
else
    print_warning "Service not running — check logs below"
    journalctl -u "${SERVICE_NAME}" -n 15 --no-pager
fi
echo "============================================"
echo ""
echo "  Directory:  $APP_DIR"
echo "  Port:       $PORT"
echo ""
echo "  Commands:"
echo "    View logs:  journalctl -u ${SERVICE_NAME} -f"
echo "    Restart:    systemctl restart ${SERVICE_NAME}"
echo "    Health:     curl http://localhost:$PORT/api/health"
echo "    Uninstall:  curl -fsSL https://raw.githubusercontent.com/$GITHUB_REPO/main/install.sh | sudo bash -s -- --uninstall"
echo "============================================"
