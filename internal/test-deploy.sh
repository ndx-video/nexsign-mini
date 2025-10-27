#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

# --- Configuration ---
# Define the hostnames of the target VMs.
# IMPORTANT: These hostnames MUST be resolvable from where you run this script.
# Add entries to your /etc/hosts file (in WSL or your Linux environment) like:
# 192.168.10.147  nsm01
# 192.168.10.174  nsm02
# 192.168.10.135  nsm03
# 192.168.10.211  nsm04
HOSTS=("nsm01" "nsm02" "nsm03" "nsm04")

SSH_USER="devops" # The user created by cloud-init
# IMPORTANT: Update this path to the location of the SSH private key
# accessible from your WSL/Git Bash environment.
SSH_KEY="/mnt/c/Users/Terence/.ssh/nsm-vbox.key"

SOURCE_DIR="./bin" # Directory containing the compiled nsm binary (relative to script location)
TARGET_BINARY="nsm" # Name of the binary file
REMOTE_INSTALL_DIR="/usr/local/bin" # Standard location for custom binaries
REMOTE_SERVICE_NAME="nsm.service" # Optional: Name of the systemd service file
REMOTE_SYSTEMD_DIR="/etc/systemd/system" # Standard location for systemd unit files

# --- Build Step ---
echo "Building nsm binary..."
# Assuming you have a Makefile or build command in your project root
# If not, replace 'make build' with your Go build command.
# Ensure the output binary is placed in the SOURCE_DIR (e.g., ./bin/nsm)
if [ -f "Makefile" ]; then
    make build
else
    # Example Go build command (adjust if your main package is elsewhere)
    mkdir -p "$SOURCE_DIR"
    go build -o "$SOURCE_DIR/$TARGET_BINARY" ./cmd/nsm
fi


if [ ! -f "$SOURCE_DIR/$TARGET_BINARY" ]; then
    echo "Build failed or binary not found at $SOURCE_DIR/$TARGET_BINARY"
    exit 1
fi

echo "Build successful: $SOURCE_DIR/$TARGET_BINARY created."

# --- Deployment Step ---
for HOST in "${HOSTS[@]}"; do
    echo "-------------------------------------"
    echo "Deploying to $HOST..."
    echo "-------------------------------------"

    # 1. Create remote install directory if it doesn't exist (using sudo)
    echo "Ensuring remote directory $REMOTE_INSTALL_DIR exists..."
    ssh -i "$SSH_KEY" "$SSH_USER@$HOST" "sudo mkdir -p $REMOTE_INSTALL_DIR"

    # 2. Copy the binary to a temporary location on the remote host
    echo "Copying binary $TARGET_BINARY to $HOST:/tmp/ ..."
    scp -i "$SSH_KEY" "$SOURCE_DIR/$TARGET_BINARY" "$SSH_USER@$HOST:/tmp/$TARGET_BINARY"

    # 3. Move binary to the final install directory and set execute permissions (using sudo)
    echo "Moving binary to $REMOTE_INSTALL_DIR and setting permissions..."
    ssh -i "$SSH_KEY" "$SSH_USER@$HOST" "sudo mv /tmp/$TARGET_BINARY $REMOTE_INSTALL_DIR/$TARGET_BINARY && sudo chmod +x $REMOTE_INSTALL_DIR/$TARGET_BINARY"

    # --- Optional: Systemd Service Management ---
    # Uncomment the following lines if you have a nsm.service file
    # and want to automate service setup/restart.
    # Assumes nsm.service file is in the same directory as this script.
    # SERVICE_FILE_PATH="./${REMOTE_SERVICE_NAME}"
    # if [ -f "$SERVICE_FILE_PATH" ]; then
    #    echo "Copying systemd service file $REMOTE_SERVICE_NAME to $HOST:/tmp/ ..."
    #    scp -i "$SSH_KEY" "$SERVICE_FILE_PATH" "$SSH_USER@$HOST:/tmp/${REMOTE_SERVICE_NAME}"
    #
    #    echo "Moving systemd service file to $REMOTE_SYSTEMD_DIR ..."
    #    ssh -i "$SSH_KEY" "$SSH_USER@$HOST" "sudo mv /tmp/${REMOTE_SERVICE_NAME} ${REMOTE_SYSTEMD_DIR}/${REMOTE_SERVICE_NAME}"
    #
    #    echo "Reloading systemd, enabling and restarting $REMOTE_SERVICE_NAME ..."
    #    ssh -i "$SSH_KEY" "$SSH_USER@$HOST" "sudo systemctl daemon-reload && sudo systemctl enable ${REMOTE_SERVICE_NAME} && sudo systemctl restart ${REMOTE_SERVICE_NAME}"
    #    echo "Service $REMOTE_SERVICE_NAME status:"
    #    ssh -i "$SSH_KEY" "$SSH_USER@$HOST" "sudo systemctl status ${REMOTE_SERVICE_NAME} --no-pager || true" # Use '|| true' to prevent script exit if service failed
    # else
    #    echo "WARNING: Systemd service file $SERVICE_FILE_PATH not found. Skipping service management."
    # fi
    # --- End Optional ---

    echo "✅ Deployment to $HOST successful."
    echo ""
done

echo "🎉 All deployments complete."