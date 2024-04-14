#!/bin/bash

# Define constants
USER="Maronato"
REPO="authifi"
TOOL_NAME="authifi"
APP_NAME="Authifi"
SERVICE_NAME="$TOOL_NAME.service"
INSTALL_DIR="$(pwd)/$TOOL_NAME"
SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME"
EXECUTABLE="$INSTALL_DIR/$TOOL_NAME"

SHOULD_SYSTEMD=0
if [[ -d "/etc/systemd/system" ]]; then
    SHOULD_SYSTEMD=1
fi


# Function to download and install the specified version of Authifi
download_and_install() {
    local version=$1
    echo "Downloading and installing $APP_NAME version $version..."

    # Determine OS and Architecture
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    case $arch in
        x86_64) arch="amd64" ;;
        arm64 | aarch64) arch="arm64" ;;
        *) echo "Unsupported architecture."; return 1 ;;
    esac

    # Construct download URLs
    local file_name="authifi-$version-$os-$arch.tar.gz"
    local file_url="https://github.com/$USER/$REPO/releases/download/$version/$file_name"
    local md5_file_url="${file_url}.md5"

    # Download and verify the files
    wget -q --show-progress "$file_url"
    wget -q --show-progress "$md5_file_url"
    echo "Verifying MD5 checksum..."
    if echo "$(cat ${file_name}.md5) ${file_name}" | md5sum -c -; then
        echo "MD5 checksum verified successfully."
        tar -xzvf "${file_name}" -C $INSTALL_DIR
        rm "${file_name}" "${file_name}.md5"
        # Update the executable
        mkdir -p $INSTALL_DIR
        return 0
    else
        echo "MD5 checksum verification failed."
        return 1
    fi
}

uninstall() {
    if [[ $SHOULD_SYSTEMD -eq 1 ]]; then
        echo "Stopping and disabling the $APP_NAME service..."
        sudo systemctl stop $SERVICE_NAME
        sudo systemctl disable $SERVICE_NAME
        echo "Removing the service file..."
        sudo rm $SERVICE_FILE
    fi
    echo "Removing installation directory..."
    rm -rf $INSTALL_DIR
    echo "$APP_NAME service has been successfully uninstalled."
}

update() {
    local version=$1

    CURRENT_VERSION=$($EXECUTABLE version)
    echo "Current installed version of $APP_NAME is: $CURRENT_VERSION"
    echo "Latest version of $APP_NAME is: $version"
    if [[ "$CURRENT_VERSION" == "$version" ]]; then
        echo "You already have the latest version of $APP_NAME installed."
        exit 0
    fi
    echo "Updating $APP_NAME..."
    if download_and_install $version; then
        if [[ $SHOULD_SYSTEMD -eq 1 ]]; then
            sudo systemctl daemon-reload
            sudo systemctl restart $SERVICE_NAME
        fi
        echo "Update successful."
    else
        echo "Update failed."
    fi
}

configure() {
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
    echo "vlans: []" >> "$DATABASE_FILE"
    echo "blocked: []" >> "$DATABASE_FILE"
}

install_service() {
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
        [Nn]* )
            echo "Service file not created. You can manually set it up later if needed."
            ;;
        * ) echo "$SERVICE_FILE_CONTENT" | sudo tee /etc/systemd/system/$SERVICE_NAME > /dev/null
            sudo systemctl enable $SERVICE_NAME
            sudo systemctl start $SERVICE_NAME
            echo "$APP_NAME service has been started successfully."
            ;;
    esac
}

# Main function
main() {

        # Inform the user about the installation directory
    echo "This script will install $APP_NAME into the local directory $INSTALL_DIR."
    read -p "Do you want to continue? (Y/n): " response
    if [[ "$response" =~ ^([nN][oO]|[nN])$ ]]
    then
        echo "Please re-execute this script from the directory you prefer to be the service's home."
        exit 1
    fi

    # Fetch the latest release data from GitHub API
    LATEST_RELEASE_INFO=$(curl -s "https://api.github.com/repos/$USER/$REPO/releases/latest")

    # Get the tag name (version) from the latest release
    LATEST_VERSION=$(echo "$LATEST_RELEASE_INFO" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    # Check if Authifi is already installed
    if [[ -f "$EXECUTABLE" ]]; then
        echo "$APP_NAME is already installed."
        read -p "Would you like to update it (1) or uninstall it (2)? " uninstall_response
        case $uninstall_response in
            1)  
                update $LATEST_VERSION

                exit 0
                ;;
            2) 
                uninstall

                exit 0
                ;;
            *) echo "Invalid option. Exiting."

                exit 1
                ;;
        esac
    fi

    SHOULD_SYSTEMD=0
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
            SHOULD_SYSTEMD=1
        fi
    fi

    # Configure Authifi
    configure

    # Download and install the latest version of Authifi
    download_and_install $LATEST_VERSION

    # Create the service file if the user chose to
    if [[ $SHOULD_SYSTEMD -eq 1 ]]; then
        install_service
    fi

    echo "$APP_NAME has been successfully installed."
}

main
