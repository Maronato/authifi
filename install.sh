#!/bin/bash

# Define constants
USER="Maronato"
REPO="authifi"
TOOL_NAME="authifi"
APP_NAME="Authifi"
SERVICE_NAME="$TOOL_NAME.service"
INSTALL_DIR="$(pwd)/$TOOL_NAME"
SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME"

# Check if the service is already installed
if systemctl is-active --quiet $SERVICE_NAME
then
    echo "$APP_NAME service is already installed. Would you like to uninstall it?"
    read -p "Uninstall the service? (Y/n): " uninstall_response
    if [[ "$uninstall_response" =~ ^([yY][eE][sS]|[yY])$ ]]
    then
        echo "Stopping and disabling the $APP_NAME service..."
        sudo systemctl stop $SERVICE_NAME
        sudo systemctl disable $SERVICE_NAME
        echo "Removing the service file..."
        sudo rm $SERVICE_FILE
        echo "Removing installation directory..."
        rm -rf $INSTALL_DIR
        echo "$APP_NAME service has been successfully uninstalled."
        exit 0
    else
        echo "Uninstall aborted. Exiting."
        exit 1
    fi
fi

# Inform the user about the installation directory
echo "This script will install $APP_NAME into the local directory $INSTALL_DIR."
read -p "Do you want to continue? (Y/n): " response
if [[ "$response" =~ ^([nN][oO]|[nN])$ ]]
then
    echo "Please re-execute this script from the directory you prefer to be the service's home."
    exit 1
fi

create_service=0
# Check if systemd is available and writable
if [[ ! -d "/etc/systemd/system" ]]; then
    echo "Systemd is not available on this system. A service file won't be created."  
else 
    if [[ ! -w "/etc/systemd/system" ]]; then
        echo "You do not have permissions to write to the systemd directory."
        echo "Please run this script with sufficient permissions to setup the service, or manually create the service file if you proceed."
        read -p "Continue without creating a service file? (y/N): " yn
        case $yn in
            [Yy]* ) ;;
            * ) exit ;;
        esac
    else
        create_service=1
    fi
fi

# Fetch the latest release data from GitHub API
LATEST_RELEASE_INFO=$(curl -s "https://api.github.com/repos/$USER/$REPO/releases/latest")

# Get the tag name (version) from the latest release
VERSION=$(echo "$LATEST_RELEASE_INFO" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

# Determine OS and Arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    arm64 | aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture."; exit 1 ;;
esac

# Get user inputs
read -p "Enter the IP address to bind to [0.0.0.0]: " BIND_ADDRESS
BIND_ADDRESS=${BIND_ADDRESS:-0.0.0.0}

read -p "Enter the port to use [1812]: " PORT
PORT=${PORT:-1812}

read -p "Enter the RADIUS secret: " RADIUS_SECRET
while [[ -z "$RADIUS_SECRET" ]]; do
    echo "RADIUS secret is required."
    read -p "Enter the RADIUS secret: " RADIUS_SECRET
done

read -p "Enter the Telegram bot token: " TELEGRAM_TOKEN
while [[ -z "$TELEGRAM_TOKEN" ]]; do
    echo "Telegram bot token is required."
    read -p "Enter the Telegram bot token: " TELEGRAM_TOKEN
done

read -p "Enter your Telegram chat ID: " TELEGRAM_CHAT_ID
while [[ -z "$TELEGRAM_CHAT_ID" ]]; do
    echo "Telegram chat ID is required."
    read -p "Enter your Telegram chat ID: " TELEGRAM_CHAT_ID
done

# Create directory and configuration file
mkdir -p $INSTALL_DIR
CONFIG_FILE="$INSTALL_DIR/config"
echo "host $BIND_ADDRESS" > "$CONFIG_FILE"
echo "port $PORT" >> "$CONFIG_FILE"
echo "radius-secret $RADIUS_SECRET" >> "$CONFIG_FILE"
echo "telegram-token $TELEGRAM_TOKEN" >> "$CONFIG_FILE"
echo "telegram-chat-ids $TELEGRAM_CHAT_ID" >> "$CONFIG_FILE"
echo "database-file $INSTALL_DIR/database.yaml" >> "$CONFIG_FILE"

# Create database file
DATABASE_FILE="$INSTALL_DIR/database.yaml"
echo "users: []" > "$DATABASE_FILE"
echo "vlans: []" > "$DATABASE_FILE"
echo "blocked: []" > "$DATABASE_FILE"

# Download and verify the file
FILE_NAME="$REPO-$VERSION-$OS-$ARCH.tar.gz"
FILE_URL="https://github.com/$USER/$REPO/releases/download/$VERSION/$FILE_NAME"
wget -q --show-progress "$FILE_URL"
wget -q --show-progress "${FILE_URL}.md5"
echo "$(cat ${FILE_NAME}.md5) ${FILE_NAME}" | md5sum -c -
if [ $? -ne 0 ]; then
    echo "MD5 checksum verification failed."
    exit 1
fi

# Extract the file and remove the tarball and checksum
tar -xzvf "${FILE_NAME}" -C "./$TOOL_NAME"

# Move the extracted directory to the installation directory
mv "$(basename "${FILE_NAME}" .tar.gz)" $INSTALL_DIR

rm "${FILE_NAME}" "${FILE_NAME}.md5"

# Prepare the service file content and prompt for creation if applicable
if [[ $create_service -eq 1 ]]; then
    SERVICE_FILE_CONTENT="[Unit]
Description=$APP_NAME Service
ConditionFileIsExecutable=$INSTALL_DIR/$TOOL_NAME
After=network.target

[Service]
StartLimitInterval=5
StartLimitBurst=10
ExecStart=$INSTALL_DIR/$TOOL_NAME serve -c $INSTALL_DIR/config
RestartSec=30

[Install]
WantedBy=multi-user.target"

    echo "This is the service file content that will be used:"
    echo "$SERVICE_FILE_CONTENT"

    read -p "Do you want to create and start the systemd service file with the above content? (Y/n): " yn
    case $yn in
        [Yy]* )
            echo "$SERVICE_FILE_CONTENT" | sudo tee /etc/systemd/system/$SERVICE_NAME > /dev/null
            sudo systemctl enable $SERVICE_NAME
            sudo systemctl start $SERVICE_NAME
            echo "$APP_NAME service has been started successfully."
            ;;
        * ) echo "Service file not created. You can manually set it up later if needed."
            ;;
    esac
else
    echo "Proceeding without creating a service file. Set it up manually if needed."
fi
