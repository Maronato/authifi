# Authifi 🛜

## Introduction
Hey there! Authifi is a simple tool for easier MAC-based VLAN assignment on Unifi routers. It’s designed to be user-friendly and straightforward, with a Telegram bot for easy control and notifications.

Every time a new device connects to your network, Authifi can automatically assign it to a default VLAN and ask you what to do next via Telegram. You can then choose to assign the device to a different VLAN for future connections or block it entirely.

## Why Authifi?
If you're like me you've probably tried to set up dynamic VLANs on your Unifi network only to find that it doesn't support default VLANs. Every time you want to add a new device, you must first find its MAC address somehow, then go to the Unifi controller and manually assign it to a VLAN. It's a hassle, especially if you want to add IOT or guest devices quickly.

Authifi solves this problem by letting you define a default VLAN for new devices and letting you quickly assign other VLANs or block devices in real-time from Telegram.

## Key Features
- **Default VLANs:** Assign new devices to a default VLAN automatically.
- **Per-Device VLANs:** Assign specific VLANs to devices on the fly.
- **Telegram Bot Integration:** Control and receive updates directly on your phone in real-time.
- **YAML Database:** Love it or hate it, it's dead simple to use and understand.
- **Lightweight:** Authifi is a tiny 8MB binary that won't even tickle your server's resources.

## Quick Start Guide
Follow these steps to get Authifi up and running with the minimum fuss:

### Step 1: Download and Install
Grab the latest release from GitHub:
```bash
wget https://github.com/maronato/authifi/releases/latest/download/authifi
chmod +x authifi
```

### Step 2: Create Your Telegram Bot
1. **Chat with BotFather:** Send `/newbot` to @BotFather on Telegram.
2. **Set a name and username for your bot,** and receive your bot token.

### Step 3: Find Your Telegram Chat ID
1. **Start your bot** by sending it a message, like `/start`.
2. **Use a bot like @userinfobot** to send `/start` and get your chat ID.

### Step 4: Configure Authifi
Create a `config.yaml` file with the necessary details:
```yaml
port: 1812
bot_token: "your_telegram_bot_token"
chat_id: "your_telegram_chat_id"
radius_secret: "your_radius_secret"
db_path: "/path/to/your/database.yaml"
```

### Step 5: Set Up Your Database
Prepare your `database.yaml`:
```yaml
users:
  - username: "user1"
    password: "password1"
    vlan: 100
vlans:
  - id: 100
    name: "Default VLAN"
    default: true
blocked:
  - username: "blocked_user"
```

### Step 6: Run Authifi
Launch Authifi with:
```bash
./authifi
```
Your server will start, and you’ll begin receiving Telegram notifications for new device connections.

### Step 7: Configure Your Gateway
Adjust your Unifi Gateway settings to use Authifi as its RADIUS server. You’ll typically need to specify the IP address and port where Authifi is running.

## Disclaimer
Authifi is a personal project and is great for individual or experimental use, but it’s not built for critical systems. Use at your own risk.

## Contributing
Feel free to dive in! Contributions, forks, and stars are all welcome.

## License
Authifi is under the MIT License. Enjoy it freely!

## Contact
Problems, questions, or high-fives? Hit up the issues on GitHub or ping me directly!

---

This step-by-step guide should make it easy for users to get started without having to dig through more detailed sections unless they want to. Let me know if there's anything else you'd like to tweak or add!
