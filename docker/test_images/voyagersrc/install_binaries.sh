#!/usr/bin/env bash

sed -i '/trap on_exit EXIT/d' /yb-voyager/installer_scripts/install-yb-voyager 
source /yb-voyager/installer_scripts/install-yb-voyager
ubuntu_install_postgres
install_debezium_server
