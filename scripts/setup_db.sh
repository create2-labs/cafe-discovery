#!/bin/bash

# Script pour vérifier et configurer la base de données MySQL/MariaDB
# Vérifie si le serveur tourne, si la DB existe et si l'utilisateur a les droits

set -e  # Arrêter en cas d'erreur
# set -x  # Décommenter pour afficher les commandes en cours d'exécution (debug)

# Couleurs pour les messages
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Fonction pour afficher les messages
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Get environment variables with default values
# Support both formats: MYSQL_* (standard) and CAFE_DB_* (legacy)
MYSQL_URL="${MYSQL_URL:-${CAFE_DB_URL:-127.0.0.1:3306}}"
MYSQL_USER="${MYSQL_USER:-${CAFE_DB_USER:-cafe}}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-${CAFE_DB_PASSWORD:-cafe}}"
MYSQL_DATABASE="${MYSQL_DATABASE:-${CAFE_DB_DATABASE_NAME:-${CAFE_DB_DATABASENAME:-cafe}}}"
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-}"

# Extract host and port from MYSQL_URL
IFS=':' read -r MYSQL_HOST MYSQL_PORT <<< "$MYSQL_URL"
MYSQL_PORT="${MYSQL_PORT:-3306}"

# Helper function to build password option for mysql command
# Returns -p"password" if password is not empty, empty string otherwise
# Note: This function outputs the option that can be used directly in mysql command
mysql_password_option() {
    local password="$1"
    if [ -n "$password" ]; then
        echo "-p$password"
    fi
}

log_info "Configuration:"
log_info "  Host: $MYSQL_HOST"
log_info "  Port: $MYSQL_PORT"
log_info "  User: $MYSQL_USER"
log_info "  Database: $MYSQL_DATABASE"


check_mysql_running() {
    local root_pwd_opt=$(mysql_password_option "$MYSQL_ROOT_PASSWORD")
    
    if mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u root $root_pwd_opt -e "SELECT 1;" >/dev/null 2>&1; then
        echo "true"   
    else
        echo "false"
    fi
}

check_database_exists() {
    local root_pwd_opt=$(mysql_password_option "$MYSQL_ROOT_PASSWORD")
    
    DB_EXISTS=$(mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u root $root_pwd_opt \
        -sN -e "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='$MYSQL_DATABASE';" 2>/dev/null || echo "0")
    
    if [ "$DB_EXISTS" = "1" ]; then
        echo "true"
    else
        echo "false"
    fi
}

check_user_exists() {
    local root_pwd_opt=$(mysql_password_option "$MYSQL_ROOT_PASSWORD")
    
    COUNT=$(mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u root $root_pwd_opt \
        -sN -e "SELECT COUNT(*) FROM mysql.user WHERE User='$MYSQL_USER' AND Host='%';" 2>/dev/null || echo "0")

    if [ "$COUNT" = "0" ] || [ -z "$COUNT" ]; then
        echo "false"
    else
        echo "true"
    fi
}

check_user_access() {
    local user_pwd_opt=$(mysql_password_option "$MYSQL_PASSWORD")

    mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" $user_pwd_opt \
        -e "USE $MYSQL_DATABASE; SELECT 1;" >/dev/null 2>&1

    if [ $? -eq 0 ]; then
        echo "true"
    else
        echo "false"
    fi
}

create_database() {
    local root_pwd_opt=$(mysql_password_option "$MYSQL_ROOT_PASSWORD")
    
    mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u root $root_pwd_opt \
        -e "CREATE DATABASE IF NOT EXISTS \`$MYSQL_DATABASE\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" 2>/dev/null
    
    if [ $? -eq 0 ]; then
        echo "true"
    else
        echo "false"
    fi
}

delete_database() {
    local root_pwd_opt=$(mysql_password_option "$MYSQL_ROOT_PASSWORD")
    
    mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u root $root_pwd_opt \
    -e "DROP DATABASE IF EXISTS \`$MYSQL_DATABASE\`;" 2>/dev/null

if [ $? -ne 0 ]; then
    log_error "Can't delete database '$MYSQL_DATABASE'"
    return 1
fi
}

create_user() {
    local root_pwd_opt=$(mysql_password_option "$MYSQL_ROOT_PASSWORD")
    
    mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u root $root_pwd_opt \
        -e "CREATE USER IF NOT EXISTS '$MYSQL_USER'@'%' IDENTIFIED BY '$MYSQL_PASSWORD';" 2>/dev/null
    
    if [ $? -eq 0 ]; then
        echo "true"
    else
        echo "false"
    fi
}

grant_privileges() {
    local root_pwd_opt=$(mysql_password_option "$MYSQL_ROOT_PASSWORD")
    
    mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u root $root_pwd_opt \
        -e "GRANT ALL PRIVILEGES ON \`$MYSQL_DATABASE\`.* TO '$MYSQL_USER'@'%';" 2>/dev/null
    
    if [ $? -ne 0 ]; then
        echo "false"
        return
    fi
    
    mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u root $root_pwd_opt \
        -e "FLUSH PRIVILEGES;" 2>/dev/null
    
    if [ $? -eq 0 ]; then
        echo "true"
    else
        echo "false"
    fi
}

if ! command -v mysql >/dev/null 2>&1; then
    log_error "MySQL client is not installed. Please install it:"
    log_error "  - macOS: brew install mysql-client"
    log_error "  - Ubuntu/Debian: sudo apt-get install mysql-client"
    log_error "  - CentOS/RHEL: sudo yum install mysql"
    exit 1
fi


MYSQL_RUNNING=$(check_mysql_running)
if [ "$MYSQL_RUNNING" != "true" ]; then
    log_error "MySQL is not running"
    log_error "Check the configuration and try again"
    exit 1
fi

DB_EXISTS=$(check_database_exists)
log_info "DB_EXISTS: $DB_EXISTS"
USER_EXISTS=$(check_user_exists)
log_info "USER_EXISTS: $USER_EXISTS"

USER_HAS_ACCESS="false"
if [ "$USER_EXISTS" = "true" ] && [ "$DB_EXISTS" = "true" ]; then
    USER_HAS_ACCESS=$(check_user_access)
    log_info "USER_HAS_ACCESS: $USER_HAS_ACCESS"
fi

if [ "$1" = "--reset-db" ]; then
    log_info "Resetting database..."
    if ! delete_database; then
        log_error "Unable to delete database"
        exit 1
    fi
    DB_EXISTS="false"
    USER_HAS_ACCESS="false"
fi

# Créer la base de données si nécessaire
if [ "$DB_EXISTS" = "false" ]; then
    log_info "Creating database '$MYSQL_DATABASE'..."
    RESULT=$(create_database)
    if [ "$RESULT" != "true" ]; then
        log_error "Unable to create database"
        exit 1
    fi
    DB_EXISTS="true"
fi

# Créer l'utilisateur si nécessaire
if [ "$USER_EXISTS" = "false" ]; then
    log_info "Creating user '$MYSQL_USER'..."
    RESULT=$(create_user)
    if [ "$RESULT" != "true" ]; then
        log_error "Unable to create user"
        exit 1
    fi
    USER_EXISTS="true"
fi

# Donner les droits si nécessaire
if [ "$USER_HAS_ACCESS" = "false" ]; then
    log_info "Granting privileges to user '$MYSQL_USER' on database '$MYSQL_DATABASE'..."
    RESULT=$(grant_privileges)
    if [ "$RESULT" != "true" ]; then
        log_error "Unable to grant privileges"
        exit 1
    fi
    # Vérifier à nouveau l'accès
    USER_HAS_ACCESS=$(check_user_access)
fi

# Vérification finale
if [ "$USER_HAS_ACCESS" = "true" ]; then
    log_info "=== Configuration completed successfully ==="
    log_info "Database '$MYSQL_DATABASE' is ready for user '$MYSQL_USER'"
else
    log_error "User '$MYSQL_USER' still does not have access to database '$MYSQL_DATABASE'"
    log_error "Please check the MySQL server logs for more information"
    exit 1
fi
