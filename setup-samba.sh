#!/bin/bash
# Setup Samba for home directory sharing
# Run this script with: sudo bash setup-samba.sh

set -e

echo "Installing Samba..."
pacman -Sy --noconfirm samba

echo "Backing up original smb.conf..."
if [ -f /etc/samba/smb.conf ]; then
    cp /etc/samba/smb.conf /etc/samba/smb.conf.backup
fi

echo "Configuring Samba for home directories..."
cat > /etc/samba/smb.conf << 'EOF'
[global]
   workgroup = WORKGROUP
   server string = %h server (Samba, Ubuntu)
   log file = /var/log/samba/log.%m
   max log size = 1000
   logging = file
   panic action = /usr/share/samba/panic-action %d
   server role = standalone server
   obey pam restrictions = yes
   unix password sync = yes
   passwd program = /usr/bin/passwd %u
   passwd chat = *Enter\snew\s*\spassword:* %n\n *Retype\snew\s*\spassword:* %n\n *password\supdated\ssuccessfully* .
   pam password change = yes
   map to guest = bad user
   usershare allow guests = no

[homes]
   comment = Home Directories
   browseable = no
   read only = no
   create mask = 0700
   directory mask = 0700
   valid users = %S
EOF

echo "Restarting Samba services..."
systemctl restart smb
systemctl restart nmb

echo "Enabling Samba services to start on boot..."
systemctl enable smb
systemctl enable nmb

echo ""
echo "Samba installation complete!"
echo ""
echo "To add your user to Samba, run:"
echo "  sudo smbpasswd -a \$USER"
echo ""
echo "Your home directory will be accessible at:"
echo "  smb://$(hostname -I | awk '{print $1}')/\$USER"
echo ""
echo "Or from Windows:"
echo "  \\\\$(hostname -I | awk '{print $1}')\\$USER"
