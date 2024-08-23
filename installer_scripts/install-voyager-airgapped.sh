#!/usr/bin/env bash

ARGS_LINUX=$@
LOG_FILE=/tmp/install-yb-voyager.log
CHECK_ONLY_DEPENDENCIES="false"
FORCE_INSTALL="false"

YB_VOYAGER_YUM_VERSION="1.7.2-0"
DEBEZIUM_YUM_VERSION="2.3.3-1.7.2"
ORA2PG_YUM_VERSION="23.2-yb.2"
YB_VOYAGER_APT_VERSION="1.7.2-0"
DEBEZIUM_APT_VERSION="2.3.3-1.7.2"
ORA2PG_APT_VERSION="23.2-yb.2"

centos_yum_package_requirements=(
  "gcc:min:0"
  "make:min:0"
  "sqlite:min:0"
  "perl:min:0"
  "perl-DBI:min:0"
  "perl-App-cpanminus:min:0"
  "perl-ExtUtils-MakeMaker:min:0"
  "mysql-devel:min:0"
  "oracle-instantclient-tools:exact:21.5.0.0.0"
  "oracle-instantclient-basic:exact:21.5.0.0.0"
  "oracle-instantclient-devel:exact:21.5.0.0.0"
  "oracle-instantclient-jdbc:exact:21.5.0.0.0"
  "oracle-instantclient-sqlplus:exact:21.5.0.0.0"
)

ubuntu_apt_package_requirements=(
  "binutils:min:2.25"
  "sqlite3:min:0"
  "gcc:min:0"
  "perl:min:0"
  "libdbi-perl:min:0"
  "libaio1:min:0"
  "cpanminus:min:0"
  "libmysqlclient-dev:min:0"
  "oracle-instantclient-tools:exact:21.5.0.0.0"
  "oracle-instantclient-basic:exact:21.5.0.0.0"
  "oracle-instantclient-devel:exact:21.5.0.0.0"
  "oracle-instantclient-jdbc:exact:21.5.0.0.0"
  "oracle-instantclient-sqlplus:exact:21.5.0.0.0"
)

cpan_modules_requirements=(
  "DBD::mysql|min|5.005"
  "Test::NoWarnings|min|1.06"
  "DBD::Oracle|min|1.83"
  "String::Random|min|0"
  "IO::Compress::Base|min|0"
)

ubuntu_missing_apt_packages=()
centos_missing_yum_packages=()
missing_cpan_modules=()
binutils_wrong_version=0
java_wrong_version=0
pg_dump_wrong_version=0
pg_restore_wrong_version=0
psql_wrong_version=0

trap on_exit EXIT

on_exit() {
	rc=$?
	set +x
	if [ $rc -eq 0 ]
	then
		echo "Done!"
	else
		echo "Script failed. Check log file ${LOG_FILE} ."
		if [[ ${ON_INSTALLER_ERROR_OUTPUT_LOG:-N} = "Y" ]]
		then
			sudo cat ${LOG_FILE}
		fi
	fi
}

main() {
    set -x

	os=$(uname -s)
	case ${os} in
		"Linux")
			get_passed_options
			# Proceed to distribution check.
			;;
		*)
			echo -e "\e[31mERROR: unsupported os ${os}\e[0m"
			exit 1
			;;
	esac

	# Linux
	dist=$(cat /etc/os-release | grep -w NAME | awk -F'=' '{print $2}' | tr -d '"')
	case ${dist} in
		"CentOS Stream")
			centos_main
			;;
		"CentOS Linux")
			centos_main
			;;
		"AlmaLinux")
			centos_main
			;;
		"Red Hat Enterprise Linux")
			centos_main
			;;
		"Red Hat Enterprise Linux Server")
			centos_main
			;;
		"Ubuntu")
			ubuntu_main
			;;
		*)
			echo -e "\e[31mERROR: unsupported linux distribution: ${dist}\e[0m"
			exit 1
			;;
	esac
    # Check if yb-voyager is installed using yb-voyager version. Else exit with error and log it too.
    yb_voyager_version=$(yb-voyager version)
    if [ $? -ne 0 ]; then
        echo -e "\e[31mERROR: yb-voyager did not get installed.\e[0m"
        exit 1
    else
        echo "yb-voyager version"
        echo "$yb_voyager_version"
    fi
}

#=============================================================================
# COMMON
#=============================================================================

output() {
	set +x
	echo "$@"
	>&2 echo "$@"
	set -x
}

check_cpan_module() {
  local module=$1
  local version_type=$2
  local required_version=$3
  local installed_version=$(perl -M"$module" -e 'print $'"$module"'::VERSION' 2>/dev/null)

  if [ -z "$installed_version" ]; then
    missing_cpan_modules+=("$module is not installed.")
  else
    case "$version_type" in
      min)
        if [ "$required_version" != "0" ] && [ "$(printf '%s\n' "$required_version" "$installed_version" | sort -V | head -n1)" != "$required_version" ]; then
          missing_cpan_modules+=("$module version is less than $required_version. Found version: $installed_version")
        fi
        ;;
      exact)
        if [ "$installed_version" != "$required_version" ]; then
          missing_cpan_modules+=("$module version is not $required_version. Found version: $installed_version")
        fi
        ;;
    esac
  fi
}

check_cpan_dependencies() {
    for module in "${cpan_modules_requirements[@]}"; do
        IFS='|' read -r cpan_module version_type required_version <<< "$module"
        check_cpan_module "$cpan_module" "$version_type" "$required_version"
    done

    if [ ${#missing_cpan_modules[@]} -ne 0 ]; then
        echo ""
        echo -e "\e[31mERROR: the following CPAN modules are missing or do not meet the version requirements:\e[0m"
        for missing in "${missing_cpan_modules[@]}"; do
            echo "$missing"
        done
    else
        echo ""
        echo "All cpan dependencies are installed and meet the version requirements."
    fi
}

check_binutils_version() {
    echo ""
	output "Checking binutils version."
	min_required_version='2.25'

	# Example output of "ld -v" on CentOS/RHEL:
	# GNU ld version 2.30-113.el8
	# Example output of "ld -v" on Ubuntu:
	# GNU ld (GNU Binutils for Ubuntu) 2.38
	version=$(ld -v | awk '{print $NF}' | awk -F '-' '{print $1}')

	version_ok=$(version_satisfied "$min_required_version" "$version")
	if [[ $version_ok -eq 0 ]]
	then
        echo ""
		echo -e "\e[31mERROR: unsupported binutils version ${version}. Update to binutils version > ${min_required_version}.\e[0m"
		binutils_wrong_version=1
	fi
}

# https://stackoverflow.com/a/4025065
# 0 if equal, 1 if $1 > $2, 2 if $2 > $1
vercomp () {
    if [[ $1 == $2 ]]
    then
        echo 0; return;
    fi
    local IFS=.
    local i ver1=($1) ver2=($2)
    # fill empty fields in ver1 with zeros
    for ((i=${#ver1[@]}; i<${#ver2[@]}; i++))
    do
        ver1[i]=0
    done
    for ((i=0; i<${#ver1[@]}; i++))
    do
        if [[ -z ${ver2[i]} ]]
        then
            # fill empty fields in ver2 with zeros
            ver2[i]=0
        fi
        if ((10#${ver1[i]} > 10#${ver2[i]}))
        then
            echo 1; return;
        fi
        if ((10#${ver1[i]} < 10#${ver2[i]}))
        then
            echo 2; return;
        fi
    done
    echo 0; return;
}

# usage: version_satisfied <minimum_version_required> <actual_version>
version_satisfied() {
    res=$(vercomp $1 $2)
    case $res in
        0) echo 1; return;;
        1) echo 0; return;;
        2) echo 1; return;;
    esac
}

check_java() {
	if [ -z "$JAVA_HOME" ]; then
		JAVA_BINARY="java"
	else
		JAVA_BINARY="$JAVA_HOME/bin/java"
	fi

    MIN_REQUIRED_MAJOR_VERSION='17'
	JAVA_COMPLETE_VERSION=$(${JAVA_BINARY} -version 2>&1 | awk -F '"' '/version/ {print $2}')
    JAVA_MAJOR_VER=$(echo "${JAVA_COMPLETE_VERSION}" | awk -F. '{print $1}')

    if ([ -n "$JAVA_MAJOR_VER" ] && (( 10#${JAVA_MAJOR_VER} >= 10#${MIN_REQUIRED_MAJOR_VERSION} )) ) #integer compare of versions.
    then
        echo ""
        output "Found sufficient java version = ${JAVA_COMPLETE_VERSION}"
    else
        echo ""
        echo -e "\e[31mERROR: Java not found or insufficient version ${JAVA_COMPLETE_VERSION}. Please install java>=${MIN_REQUIRED_MAJOR_VERSION}\e[0m"
        java_wrong_version=1
    fi
}

get_passed_options() {
	OPTS=$(getopt -o "df", --long check-only-dependencies,force-install --name 'install-voyager-airgapped' -- $ARGS_LINUX)

	eval set -- "$OPTS"

	while true; do
		case "$1" in
			-d | --check-only-dependencies ) 
				CHECK_ONLY_DEPENDENCIES="true";
				shift
				;;
            -f | --force-install )
                FORCE_INSTALL="true"
				shift
				;;
			* ) 
				break 
				;;
		esac
	done
}

check_pg_dump_and_pg_restore_version() {
    # Check if pg_dump and pg_restore are installed and their major versions are greater than min required version
    MIN_REQUIRED_MAJOR_VERSION='14'
    PG_DUMP_VERSION=$(pg_dump --version | awk '{print $3}')
    PG_RESTORE_VERSION=$(pg_restore --version | awk '{print $3}')
    PSQL_VERSION=$(psql --version | awk '{print $3}')
    PG_DUMP_MAJOR_VER=$(echo "${PG_DUMP_VERSION}" | awk -F. '{print $1}')
    PG_RESTORE_MAJOR_VER=$(echo "${PG_RESTORE_VERSION}" | awk -F. '{print $1}')
    PSQL_MAJOR_VER=$(echo "${PSQL_VERSION}" | awk -F. '{print $1}')

    # Check them separately
    if ([ -n "$PG_DUMP_MAJOR_VER" ] && (( 10#${PG_DUMP_MAJOR_VER} >= 10#${MIN_REQUIRED_MAJOR_VERSION} )) ) #integer compare of versions.
    then
        echo ""
        output "Found sufficient pg_dump version = ${PG_DUMP_VERSION}"
    else
        echo ""
        echo -e "\e[31mERROR: pg_dump not found or insufficient version ${PG_DUMP_VERSION}. Please install pg_dump>=${MIN_REQUIRED_MAJOR_VERSION}\e[0m"
        pg_dump_wrong_version=1
    fi

    if ([ -n "$PG_RESTORE_MAJOR_VER" ] && (( 10#${PG_RESTORE_MAJOR_VER} >= 10#${MIN_REQUIRED_MAJOR_VERSION} )) ) #integer compare of versions.
    then
        echo ""
        output "Found sufficient pg_restore version = ${PG_RESTORE_VERSION}"
    else
        echo ""
        echo -e "\e[31mERROR: pg_restore not found or insufficient version ${PG_RESTORE_VERSION}. Please install pg_restore>=${MIN_REQUIRED_MAJOR_VERSION}\e[0m"
        pg_restore_wrong_version=1
    fi

    if ([ -n "$PSQL_MAJOR_VER" ] && (( 10#${PSQL_MAJOR_VER} >= 10#${MIN_REQUIRED_MAJOR_VERSION} )) ) #integer compare of versions.
    then
        echo ""
        output "Found sufficient psql version = ${PSQL_VERSION}"
    else
        echo ""
        echo -e "\e[31mERROR: psql not found or insufficient version ${PSQL_VERSION}. Please install psql>=${MIN_REQUIRED_MAJOR_VERSION}\e[0m"
        psql_wrong_version=1
    fi
}

#=============================================================================
# CENTOS/RHEL
#=============================================================================

centos_main() {
    # If --force-install is not passed, check dependencies.
    if [ "$FORCE_INSTALL" = "false" ]; then
        echo "Checking dependencies..."
        check_binutils_version
        check_java
        check_pg_dump_and_pg_restore_version
        check_yum_dependencies
        check_cpan_dependencies
        # If either of the yum or cpan dependencies are missing or binutils wrong version, exit with error.
        if { [ ${#centos_missing_yum_packages[@]} -ne 0 ] || [ ${#missing_cpan_modules[@]} -ne 0 ] || [ "$binutils_wrong_version" -eq 1 ] || [ "$java_wrong_version" -eq 1 ] || [ "$pg_dump_wrong_version" -eq 1 ] || [ "$pg_restore_wrong_version" -eq 1 ] || [ "$psql_wrong_version" -eq 1 ]; } && [ "$FORCE_INSTALL" = "false" ]; then 
            echo ""
            echo -e "\e[33mThe script searches for specific package names only. If similar packages are not detected but are present and deemed reliable, use --force-install to install Voyager.\e[0m"
            exit 1
        fi

        if [ "$CHECK_ONLY_DEPENDENCIES" = "true" ]; then
            echo ""
            echo "All dependencies are satisfied."
            exit 0
        fi
    else
        echo ""
        echo -e "\e[33mForce install option is passed. Skipping dependency checks and proceeding with installation.\e[0m"
    fi

    echo ""
    echo "Installing ora2pg..."
    sudo yum install -y -q ora2pg-"$ORA2PG_YUM_VERSION".noarch.rpm 1>&2 
    if [ $? -ne 0 ]; then
        echo ""
        echo -e "\e[31mERROR: ora2pg did not get installed.\e[0m"
        exit 1
    fi
    echo ""
    echo "Installing debezium..."
    sudo yum install -y -q debezium-"$DEBEZIUM_YUM_VERSION".noarch.rpm 1>&2
    if [ $? -ne 0 ]; then
        echo ""
        echo -e "\e[31mERROR: debezium did not get installed.\e[0m"
        exit 1
    fi
    echo ""
    echo "Installing yb-voyager..."
    sudo yum install -y -q yb-voyager-"$YB_VOYAGER_YUM_VERSION".x86_64.rpm 1>&2
    if [ $? -ne 0 ]; then
        echo ""
        echo -e "\e[31mERROR: yb-voyager did not get installed.\e[0m"
        exit 1
    fi
    echo ""
    echo "Installation completed." 

    set +x 
}

check_yum_package_version() {
    local package=$1
    local version_type=$2
    local required_version=$3
    local installed_version=$(yum list installed "$package" 2>/dev/null | awk '/^Installed Packages/ {getline; print $2}' | cut -d- -f1)

    if [ -z "$installed_version" ]; then
        centos_missing_yum_packages+=("$package is not installed.")
    else
        case "$version_type" in
        min)
            if [ "$required_version" != "0" ] && [ "$(printf '%s\n' "$required_version" "$installed_version" | sort -V | head -n1)" != "$required_version" ]; then
                centos_missing_yum_packages+=("$package version is less than $required_version. Found version: $installed_version")
            fi
        ;;
        exact)
            # If package name starts with oracle-instantclient then convert installed version from example 21.5.0.0.0-1 to 21.5.0.0.0
            if [[ "$package" =~ ^oracle-instantclient ]]; then
                installed_version=$(echo "$installed_version" | cut -d'-' -f1)
            fi
            if [ "$installed_version" != "$required_version" ]; then
                centos_missing_yum_packages+=("$package version is not $required_version. Found version: $installed_version")
            fi
        ;;
        esac
    fi
}

check_yum_dependencies() {
    for requirement in "${centos_yum_package_requirements[@]}"; do
        IFS=":" read -r package version_type required_version <<< "$requirement"
        check_yum_package_version "$package" "$version_type" "$required_version"
    done

    if [ ${#centos_missing_yum_packages[@]} -ne 0 ]; then
        echo ""
        echo -e "\e[31mERROR: the following yum packages are missing or do not meet the version requirements:\e[0m"
        for missing in "${centos_missing_yum_packages[@]}"; do
            echo "$missing"
        done
    else
        echo ""
        echo "All yum dependencies are installed and meet the version requirements."
    fi
}

#=============================================================================
# UBUNTU
#=============================================================================

ubuntu_main() {
    # If --force-install is not passed, check dependencies.
    if [ "$FORCE_INSTALL" = "false" ]; then
        echo "Checking dependencies..."
        check_binutils_version
        check_java
        check_pg_dump_and_pg_restore_version
        check_apt_dependencies
        check_cpan_dependencies

        # If either of the apt or cpan dependencies are missing or binutils wrong version, exit with error.
        if { [ ${#ubuntu_missing_apt_packages[@]} -ne 0 ] || [ ${#missing_cpan_modules[@]} -ne 0 ] || [ "$binutils_wrong_version" -eq 1 ] || [ "$java_wrong_version" -eq 1 ] || [ "$pg_dump_wrong_version" -eq 1 ] || [ "$pg_restore_wrong_version" -eq 1 ] || [ "$psql_wrong_version" -eq 1 ]; } && [ "$FORCE_INSTALL" = "false" ]; then
            echo ""
            echo -e "\e[33mWe search for specific package names only. If you have any similar packages installed which are not getting detected by us and you are confident in them, you can install voyager anyway using the --force-install option.\e[0m"
            exit 1
        fi
    
        if [ "$CHECK_ONLY_DEPENDENCIES" = "true" ]; then
            echo ""
            echo "All dependencies are satisfied."
            exit 0
        fi
    else 
        echo ""
        echo "Force install option is passed. Proceeding with installation."
    fi

    echo ""
    echo "Installing ora2pg..."
    sudo apt install -y -q ./ora2pg_"$ORA2PG_YUM_VERSION"_all.deb 1>&2
    if [ $? -ne 0 ]; then
        echo ""
        echo -e "\e[31mERROR: ora2pg did not get installed.\e[0m"
        exit 1
    fi
    echo ""
    echo "Installing debezium..."
    sudo apt install -y -q ./debezium_"$DEBEZIUM_YUM_VERSION"_all.deb 1>&2
    if [ $? -ne 0 ]; then
        echo ""
        echo -e "\e[31mERROR: debezium did not get installed.\e[0m"
        exit 1
    fi
    echo ""
    echo "Installing yb-voyager..."
    sudo apt install -y -q ./yb-voyager_"$YB_VOYAGER_YUM_VERSION"_amd64.deb 1>&2
    if [ $? -ne 0 ]; then
        echo ""
        echo -e "\e[31mERROR: yb-voyager did not get installed.\e[0m"
        exit 1
    fi
    echo ""
    echo "Installation completed."

    set +x
}

check_apt_dependencies() {
    for requirement in "${ubuntu_apt_package_requirements[@]}"; do
        IFS=":" read -r package version_type required_version <<< "$requirement"
        check_apt_package_version "$package" "$version_type" "$required_version"
    done

    if [ ${#ubuntu_missing_apt_packages[@]} -ne 0 ]; then
        echo ""
        echo -e "\e[31mERROR: the following apt packages are missing or do not meet the version requirements:\e[0m"
        for missing in "${ubuntu_missing_apt_packages[@]}"; do
            echo "$missing"
        done
    else
        echo ""
        echo "All apt dependencies are installed and meet the version requirements."
    fi
}

check_apt_package_version() {
    local package=$1
    local version_type=$2
    local required_version=$3
    local installed_version=$(dpkg-query -f '${Version}' -W "$package" 2>/dev/null)

    if [ -z "$installed_version" ]; then
        ubuntu_missing_apt_packages+=("$package is not installed.")
    else
        case "$version_type" in
        min)
            if [ "$required_version" != "0" ] && [ "$(printf '%s\n' "$required_version" "$installed_version" | sort -V | head -n1)" != "$required_version" ]; then
                ubuntu_missing_apt_packages+=("$package version is less than $required_version. Found version: $installed_version")
            fi
        ;;
        exact)
            # If package name starts with oracle-instantclient then convert installed version from example 21.5.0.0.0-1 to 21.5.0.0.0
            if [[ "$package" =~ ^oracle-instantclient ]]; then
                installed_version=$(echo "$installed_version" | cut -d'-' -f1)
            fi
            if [ "$installed_version" != "$required_version" ]; then
                ubuntu_missing_apt_packages+=("$package version is not $required_version. Found version: $installed_version")
            fi
        ;;
        esac
    fi
}

main 2>> $LOG_FILE
{
  set +x; set -e 
} 2> /dev/null


