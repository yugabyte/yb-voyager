#!/usr/bin/env bash

ORA2PG_VERSION="23.1"

GO_MIN_VERSION="go1.17"
GO_VERSION="go1.18"

PG_DUMP_MIN_VERSION="pg_dump (PostgreSQL) 10"
PG_DUMP_VERSION="pg_dump (PostgreSQL) 14"

PERL_MIN_VERSION="5.10"

DBI_MIN_VERSION="1.614"

# truncate the log file
> migration_installer.log

# separate file for bashrc related settings
touch $HOME/.migration_installer_bashrc

# just in case last execution of script stopped in between
source $HOME/.migration_installer_bashrc

install_tool() {
    tool=$1
    which ${tool} &>> migration_installer.log
    if [ $? -ne 0 ]; then
        echo "${tool} not found, installing ${tool}...";
            sudo yum install -y ${tool} >> migration_installer.log; 
            if [ $? -ne 0 ]; then
                echo "${tool} not installed! Check migration_installer.log for more details."
                exit
            fi
        echo "Installed: ${tool}"
    else
        echo "${tool} is already installed. Skipping."
    fi
}

install_tool which
install_tool wget
install_tool git
install_tool gcc

which go &>> migration_installer.log
if [ $? -ne 0 ]; then
    echo "go not found."
    while true; do
        read -p "Install GoLang? (Y/N): " yn
        case $yn in
            [Yy]* ) 
            echo "Installing GoLang."
            wget https://golang.org/dl/${GO_VERSION}.linux-amd64.tar.gz &>> migration_installer.log
            if [ $? -ne 0 ]; then
                echo "GoLang not installed! Check migration_installer.log for more details."
                exit
            fi
            sudo tar -C /usr/local -xzf ${GO_VERSION}.linux-amd64.tar.gz &>> migration_installer.log
            if [ $? -ne 0 ]; then
                echo "GoLang not installed! Check migration_installer.log for more details."
                exit
            fi
            rm ${GO_VERSION}.linux-amd64.tar.gz
            echo 'export PATH=$PATH:/usr/local/go/bin' >> $HOME/.migration_installer_bashrc
            echo 'export GOPATH=$HOME/go' >> $HOME/.migration_installer_bashrc
            echo 'export PATH=$PATH:$GOPATH/bin' >> $HOME/.migration_installer_bashrc
            echo "Golang Installed."
            break;;
            [Nn]* ) echo "Cannot proceed without go. Exiting....."; exit;;
            * ) echo "Please answer y/Y or n/N.";;
        esac
    done
else
    GO_INSTALLED_VERSION=$(go version | cut -d " " -f 3)    
    if [ "$GO_INSTALLED_VERSION" \> "$GO_MIN_VERSION" ] || [ "$GO_INSTALLED_VERSION" = "$GO_MIN_VERSION" ] ; then 
        echo "$GO_INSTALLED_VERSION is already installed. Skipping."
    else 
        echo -e "Installed $GO_INSTALLED_VERSION version is below the minimum required version $GO_MIN_VERSION.\nPlease uninstall current $GO_INSTALLED_VERSION and rerun this scirpt."
        exit
    fi
fi
source $HOME/.migration_installer_bashrc


which pg_dump &>> migration_installer.log
if [ $? -ne 0 ]; then
    echo "pg_dump not found."
    while true; do
        read -p "Install Postgres14? (Y/N): " yn
        case $yn in
            [Yy]* ) 
            echo "Installing Postgres14"
            sudo yum install -y https://download.postgresql.org/pub/repos/yum/reporpms/EL-7-x86_64/pgdg-redhat-repo-latest.noarch.rpm >> migration_installer.log
            if [ $? -ne 0 ]; then
                echo "PostgreSQL not installed! Check migration_installer.log for more details."
                exit
            fi
            
            sudo yum install -y postgresql14-server &>> migration_installer.log
            if [ $? -ne 0 ]; then
                echo "PostgreSQL not installed! Check migration_installer.log for more details."
                exit
            fi
            echo "Installed: Postgres14"
            break;;
            [Nn]* ) echo "User denied postgress installation. Skipping...."; break;;
            * ) echo "Please answer y/Y or n/N.";;
        esac
    done
else
    PG_DUMP_INSTALLED_VERSION=$(pg_dump --version)
    if [ "$PG_DUMP_INSTALLED_VERSION" \> "$PG_DUMP_MIN_VERSION" ] || [ "$PG_DUMP_INSTALLED_VERSION" = "$PG_DUMP_MIN_VERSION" ] ; then 
        echo "$PG_DUMP_INSTALLED_VERSION is already installed. Skipping."
    else 
        echo -e "Installed $PG_DUMP_INSTALLED_VERSION version is below the minimum required version $PG_DUMP_MIN_VERSION.\nPlease uninstall current $PG_DUMP_INSTALLED_VERSION and rerun this scirpt."
        exit
    fi
fi

# TODO which oracle client version to install? Latest?
while true; do
    read -p "Install Oracle Instant Clients (Required for ora2pg tool)? (Y/N): " yn
    case $yn in
        [Yy]* ) 
        echo "Installing Oracle Instant Clients"
        wget https://download.oracle.com/otn_software/linux/instantclient/215000/oracle-instantclient-basic-21.5.0.0.0-1.x86_64.rpm &>> migration_installer.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check migration_installer.log for more details."
            exit
        fi
        sudo yum install -y oracle-instantclient-basic-21.5.0.0.0-1.x86_64.rpm &>> migration_installer.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check migration_installer.log for more details."
            exit
        fi
        rm oracle-instantclient-basic-21.5.0.0.0-1.x86_64.rpm

        wget https://download.oracle.com/otn_software/linux/instantclient/215000/oracle-instantclient-devel-21.5.0.0.0-1.x86_64.rpm &>> migration_installer.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check migration_installer.log for more details."
            exit
        fi
        sudo yum install -y oracle-instantclient-devel-21.5.0.0.0-1.x86_64.rpm &>> migration_installer.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check migration_installer.log for more details."
            exit
        fi
        rm oracle-instantclient-devel-21.5.0.0.0-1.x86_64.rpm

        wget https://download.oracle.com/otn_software/linux/instantclient/215000/oracle-instantclient-jdbc-21.5.0.0.0-1.x86_64.rpm &>> migration_installer.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check migration_installer.log for more details."
            exit
        fi
        sudo yum install -y oracle-instantclient-jdbc-21.5.0.0.0-1.x86_64.rpm &>> migration_installer.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check migration_installer.log for more details."
            exit
        fi
        rm oracle-instantclient-jdbc-21.5.0.0.0-1.x86_64.rpm

        wget https://download.oracle.com/otn_software/linux/instantclient/215000/oracle-instantclient-sqlplus-21.5.0.0.0-1.x86_64.rpm &>> migration_installer.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check migration_installer.log for more details."
            exit
        fi
        sudo yum install -y oracle-instantclient-sqlplus-21.5.0.0.0-1.x86_64.rpm &>> migration_installer.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check migration_installer.log for more details."
            exit
        fi
        rm oracle-instantclient-sqlplus-21.5.0.0.0-1.x86_64.rpm

        break;;
        [Nn]* ) echo "User denied instant-client installation. Skipping....."; break;;
        * ) echo "Please answer y/Y or n/N.";;
    esac
done

if [ -z ${ORACLE_HOME+x} ]; then
    while true; do
        read -p "ORACLE_HOME not set. Do you want to set it to /usr/lib/oracle/21/client64? (Y/N): " yn
        case $yn in
            [Yy]* )
            echo 'export ORACLE_HOME=/usr/lib/oracle/21/client64' >> $HOME/.migration_installer_bashrc
            break;;
            [Nn]* )
            echo "Skipping...."
            break;;
            * ) echo "Please answer y/Y or n/N.";;
        esac
    done
else
    echo "ORACLE_HOME already set to $ORACLE_HOME. Skipping...."
fi
source $HOME/.migration_installer_bashrc

if [ -z ${LD_LIBRARY_PATH+x} ]; then
    while true; do
        read -p "LD_LIBRARY_PATH not set. Do you want to set it to $ORACLE_HOME/lib:$LD_LIBRARY_PATH? (Y/N): " yn
        case $yn in
            [Yy]* )
            echo 'export LD_LIBRARY_PATH=$ORACLE_HOME/lib:$LD_LIBRARY_PATH' >> $HOME/.migration_installer_bashrc
            break;;
            [Nn]* )
            echo "Skipping...."
            break;;
            * ) echo "Please answer y/Y or n/N.";;
        esac
    done
else
    echo "LD_LIBRARY_PATH already set to $LD_LIBRARY_PATH. Skipping...."
fi
source $HOME/.migration_installer_bashrc

while true; do
    read -p "Do you want to include \$ORACLE_HOME/bin($ORACLE_HOME/bin) to you PATH variable? (Y/N): " yn
    case $yn in
        [Yy]* )
        echo 'export PATH=$PATH:$ORACLE_HOME/bin' >> $HOME/.migration_installer_bashrc
        break;;
        [Nn]* )
        echo "Skipping...."
        break;;
        * ) echo "Please answer y/Y or n/N.";;
    esac
done
source $HOME/.migration_installer_bashrc

ora2pg_license_acknowledgement() {
cat << EndOfText
 ---------------------------------------------------------------------------------------------------------------
|                                                        IMPORTANT                                              |
| Ora2Pg is licensed under the GNU General Public License available at https://ora2pg.darold.net/license.html   |                   
| By indicating "I Accept" through an affirming action, you indicate that you accept the terms                  | 
| of the GNU General Public License  Agreement and you also acknowledge that you have the authority,            |
| on behalf of your company, to bind your company to such terms.                                                |
| You may then download or install the file.                                                                    |
 ---------------------------------------------------------------------------------------------------------------
EndOfText

while true; do    
    read -p "Do you Accept? (Y/N)" yn
    case $yn in
        [Yy]* )
        break;;
        [Nn]* )
        exit;;
        * ) ;;
    esac
done
}

which ora2pg &>> migration_installer.log
if  [ $? -ne 0 ]; then
    echo "ora2pg not found."
    while true; do
        read -p "Install ora2pg? (Y/N): " yn
        case $yn in
            [Yy]* )
            ora2pg_license_acknowledgement
            perl_version=$(perl --version | sed -n '2p' | cut -d "(" -f2 | cut -d ")" -f1 | cut -d "v" -f2)
            if [ -z $perl_version ]; then
                echo "perl not found."
                echo "Installing perl."
                sudo yum install -y perl &>> migration_installer.log
                if [ $? -ne 0 ]; then
                    echo "perl not installed! Check migration_installer.log for more details."
                    exit
                fi
                echo "perl installed."
            elif [[ "$perl_version" < "$PERL_MIN_VERSION" ]]; then
                echo "perl Found, But atleast 5.10 version required."
                echo "Installing latest perl."
                sudo yum install -y perl &>> migration_installer.log
                if [ $? -ne 0 ]; then
                    echo "perl not installed! Check migration_installer.log for more details."
                    exit
                fi
                echo "perl installed."
            else
                echo "perl ${perl_version} is already installed. Skipping."
            fi

            perl -e 'use DBI' &>> migration_installer.log
            if [ $? -ne 0 ]; then
                echo "DBI not found."
                echo "Installing DBI..."
                sudo yum install -y perl-DBI &>> migration_installer.log
                if [ $? -ne 0 ]; then
                    echo "DBI not installed! Check migration_installer.log for more details."
                    exit
                fi
                echo "DBI installed."
            else
                dbi_version=$(perl -e 'use DBI; DBI->installed_versions();' | sed -n /DBI/p | cut -d ':' -f2 | cut -d ' ' -f2)
                if [[ "$dbi_version" < "$DBI_MIN_VERSION" ]]; then
                    echo "DBI Found, But atleast 1.614 version required."
                    echo "Installing latest DBI."
                    sudo yum install -y perl &>> migration_installer.log
                    if [ $? -ne 0 ]; then
                        echo "DBI not installed! Check migration_installer.log for more details."
                        exit
                    fi
                    echo "DBI installed."
                else
                    echo "perl-DBI (1.614 or more) is already installed. Skipping."
                fi
            fi

            which cpanm &>> migration_installer.log
            if [ $? -ne 0 ]; then
                echo "cpanm not found."
                echo "Installing cpanm."
                sudo yum install -y perl-App-cpanminus &>> migration_installer.log
                if [ $? -ne 0 ]; then
                    echo "cpanm not installed! Check migration_installer.log for more details."
                    exit
                fi
                echo "cpanm installed."
            else
                echo "cpanm is already installed. Skipping."
            fi

            dbd_mysql_version=$(perl -e 'use DBI; DBI->installed_versions();' | sed -n /DBD::mysql/p | cut -d ':' -f4 | cut -d ' ' -f2)
            if [ -z $dbd_mysql_version  ]; then
                echo "DBD::mysql not found."
                echo "Installing DBD::mysql."
                sudo yum install -y mysql-devel &>> migration_installer.log
                if [ $? -ne 0 ]; then
                    echo "mysql-devel not installed! Check migration_installer.log for more details."
                    exit
                fi
                sudo cpanm DBD::mysql &>> migration_installer.log
                if [ $? -ne 0 ]; then
                    echo "DBD::mysql not installed! Check migration_installer.log for more details."
                    exit
                fi
                echo "DBD::mysql installed."
            else
                echo "DBD::mysql is already installed. Skipping."
            fi

            dbd_oracle_version=$(perl -e 'use DBI; DBI->installed_versions();' | sed -n /DBD::Oracle/p | cut -d ':' -f4 | cut -d ' ' -f2)
            if [ -z $dbd_oracle_version  ]; then
                echo "DBD::Oracle not found."
                echo "Installing DBD::Oracle."
                sudo cpanm Test::NoWarnings &>> migration_installer.log
                if [ $? -ne 0 ]; then
                    echo "DBD::Oracle not installed! Check migration_installer.log for more details."
                    exit
                fi
                sudo cpanm DBD::Oracle &>> migration_installer.log
                if [ $? -ne 0 ]; then
                    echo "DBD::Oracle not installed! Check migration_installer.log for more details."
                    exit
                fi
                echo "DBD::Oracle installed."
            else
                echo "DBD::Oracle is already installed. Skipping."
            fi

            wget https://github.com/darold/ora2pg/archive/refs/tags/v${ORA2PG_VERSION}.tar.gz &>> migration_installer.log
            if [ $? -ne 0 ]; then
                echo "ora2pg not installed! Check migration_installer.log for more details."
                exit
            fi
            tar -xvf v${ORA2PG_VERSION}.tar.gz &>> migration_installer.log
            cd ora2pg-${ORA2PG_VERSION}/
            perl Makefile.PL &>> migration_installer.log
            if [ $? -ne 0 ]; then
                echo "ora2pg not installed! Check migration_installer.log for more details."
                exit
            fi
            make &>> migration_installer.log && sudo make install &>> migration_installer.log
            if [ $? -ne 0 ]; then
                echo "ora2pg not installed! Check migration_installer.log for more details."
                exit
            fi
            cd ~
            rm -f v${ORA2PG_VERSION}.tar.gz
            echo "ora2pg installed."
            break;;
            [Nn]* )
            echo "User denied installation. Skipping......"
            break;;
            * ) echo "Please answer Y/y or N/n."
        esac
    done
else
    echo "ora2pg is already installed. Skipping."
fi

while true; do
    echo "*** File - $HOME/.migration_installer_bashrc contents ***"
    cat $HOME/.migration_installer_bashrc
    read -p "Add $HOME/.migration_installer_bashrc to $HOME/.bashrc file (Y/N)?" yn
    case $yn in
    [Yy]* )
    echo 'source $HOME/.migration_installer_bashrc' >> $HOME/.bashrc
    echo "execute: \"source $HOME/.bashrc\" before continuing in same shell"
    break;;
    [Nn]* )
    echo "execute: \"source $HOME/.migration_installer_bashrc\" before continuing to have paths set in current shell"
    break;;
    * ) ;;
    esac
done