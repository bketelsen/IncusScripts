#!/usr/bin/env bash

# Copyright (c) 2021-2025 community-scripts ORG
# Author: jkrgr0
# License: MIT | https://github.com/community-scripts/ProxmoxVE/raw/main/LICENSE
# Source: https://docs.2fauth.app/

source /dev/stdin <<<"$FUNCTIONS_FILE_PATH"
color
verb_ip6
catch_errors
setting_up_container
network_check
update_os

msg_info "Installing Dependencies"
$STD apt-get install -y \
  lsb-release
curl -fsSL https://packages.sury.org/php/apt.gpg | gpg --dearmor -o /usr/share/keyrings/deb.sury.org-php.gpg
echo "deb [signed-by=/usr/share/keyrings/deb.sury.org-php.gpg] https://packages.sury.org/php/ $(lsb_release -sc) main" >/etc/apt/sources.list.d/php.list
$STD apt-get update

$STD apt-get install -y \
  nginx \
  composer \
  php8.3-{bcmath,common,ctype,curl,fileinfo,fpm,gd,intl,mbstring,mysql,xml,cli}
msg_ok "Installed Dependencies"

install_mariadb

msg_info "Setting up Database"
DB_NAME=2fauth_db
DB_USER=2fauth
DB_PASS=$(openssl rand -base64 18 | tr -dc 'a-zA-Z0-9' | head -c13)
$STD mariadb -u root -e "CREATE DATABASE $DB_NAME;"
$STD mariadb -u root -e "CREATE USER '$DB_USER'@'localhost' IDENTIFIED BY '$DB_PASS';"
$STD mariadb -u root -e "GRANT ALL ON $DB_NAME.* TO '$DB_USER'@'localhost'; FLUSH PRIVILEGES;"
{
  echo "2FAuth Credentials"
  echo "Database User: $DB_USER"
  echo "Database Password: $DB_PASS"
  echo "Database Name: $DB_NAME"
} >>~/2FAuth.creds
msg_ok "Set up Database"

msg_info "Setup 2FAuth"
RELEASE=$(curl -fsSL https://api.github.com/repos/Bubka/2FAuth/releases/latest | grep "tag_name" | awk '{print substr($2, 2, length($2)-3) }')
curl -fsSL "https://github.com/Bubka/2FAuth/archive/refs/tags/${RELEASE}.zip" -o "${RELEASE}.zip"
$STD unzip "${RELEASE}.zip"
mv "2FAuth-${RELEASE//v/}/" /opt/2fauth

cd "/opt/2fauth" || return
cp .env.example .env
IPADDRESS=$(hostname -I | awk '{print $1}')

sed -i -e "s|^APP_URL=.*|APP_URL=http://$IPADDRESS|" \
  -e "s|^DB_CONNECTION=$|DB_CONNECTION=mysql|" \
  -e "s|^DB_DATABASE=$|DB_DATABASE=$DB_NAME|" \
  -e "s|^DB_HOST=$|DB_HOST=127.0.0.1|" \
  -e "s|^DB_PORT=$|DB_PORT=3306|" \
  -e "s|^DB_USERNAME=$|DB_USERNAME=$DB_USER|" \
  -e "s|^DB_PASSWORD=$|DB_PASSWORD=$DB_PASS|" .env

export COMPOSER_ALLOW_SUPERUSER=1
$STD composer update --no-plugins --no-scripts
$STD composer install --no-dev --prefer-source --no-plugins --no-scripts

$STD php artisan key:generate --force

$STD php artisan migrate:refresh
$STD php artisan passport:install -q -n
$STD php artisan storage:link
$STD php artisan config:cache

chown -R www-data: /opt/2fauth
chmod -R 755 /opt/2fauth

echo "${RELEASE}" >"/opt/2fauth_version.txt"
msg_ok "Setup 2fauth"

msg_info "Configure Service"
cat <<EOF >/etc/nginx/conf.d/2fauth.conf
server {
        listen 80;
        root /opt/2fauth/public;
        server_name $IPADDRESS;
        index index.php;
        charset utf-8;

        location / {
                try_files \$uri \$uri/ /index.php?\$query_string;
        }

        location = /favicon.ico { access_log off; log_not_found off; }
        location = /robots.txt { access_log off; log_not_found off; }

        error_page 404 /index.php;

        location ~ \.php\$ {
                fastcgi_pass unix:/var/run/php/php8.3-fpm.sock;
                fastcgi_param SCRIPT_FILENAME \$realpath_root\$fastcgi_script_name;
                include fastcgi_params;
        }

        location ~ /\.(?!well-known).* {
                deny all;
        }
}
EOF

systemctl reload nginx
msg_ok "Configured Service"

motd_ssh
customize

msg_info "Cleaning up"
rm -f "/opt/v${RELEASE}.zip"
$STD apt-get -y autoremove
$STD apt-get -y autoclean
msg_ok "Cleaned"

# Modified by surgeon https://github.com/bketelsen/surgeon