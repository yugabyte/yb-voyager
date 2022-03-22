#!/usr/bin/env bash

which wget &> temp.log
if [ $? -ne 0 ]; then
    echo "wget not found."
    while true; do
        read -p "Install wget? (Y/N): " yn
        case $yn in
            [Yy]* ) 
            echo "Installing wget.";
            sudo yum install wget -y >> mylog.log; 
            if [ $? -ne 0 ]; then
                echo "wget not installed! Check $HOME/mylog.log for more details."
                exit
            fi
            echo "Installed wget."
            break;;
            [Nn]* ) echo "Cannot proceed without wget. Exiting...."; exit;;
            * ) echo "Please answer y/Y or n/N.";;
        esac
    done
else
    echo "wget is already installed. Skipping."
fi

which go &>> temp.log
if [ $? -ne 0 ]; then
    echo "go not found."
    while true; do
        read -p "Install GoLang? (Y/N): " yn
        case $yn in
            [Yy]* ) 
            echo "Installing GoLang."
            wget https://golang.org/dl/go1.17.2.linux-amd64.tar.gz &>> mylog.log
            if [ $? -ne 0 ]; then
                echo "GoLang not installed! Check $HOME/mylog.log for more details."
                exit
            fi
            sudo tar -C /usr/local -xzf go1.17.2.linux-amd64.tar.gz &>> mylog.log
            if [ $? -ne 0 ]; then
                echo "GoLang not installed! Check $HOME/mylog.log for more details."
                exit
            fi
            rm go1.17.2.linux-amd64.tar.gz
            echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
            export PATH=$PATH:/usr/local/go/bin
            echo 'export GOPATH=$HOME/go' >> ~/.bashrc
            export GOPATH=$HOME/go
            echo 'export PATH=$PATH:$GOPATH/bin' >> ~/.bashrc
            export PATH=$PATH:$GOPATH/bin
            echo "GoLang Installed."
            break;;
            [Nn]* ) echo "Cannot proceed without go. Exiting....."; exit;;
            * ) echo "Please answer y/Y or n/N.";;
        esac
    done
else
    echo "go is already installed. Skipping."
fi
source ~/.bashrc

which pg_dump &>> temp.log
if [ $? -ne 0 ]; then
    echo "pg_dump not found."
    while true; do
        read -p "Install Postgres11? (Y/N): " yn
        case $yn in
            [Yy]* ) 
            echo "Installing Postgress11."
            sudo yum install -y https://download.postgresql.org/pub/repos/yum/reporpms/EL-7-x86_64/pgdg-redhat-repo-latest.noarch.rpm >> mylog.log
            if [ $? -ne 0 ]; then
                echo "PostgreSQL not installed! Check $HOME/mylog.log for more details."
                exit
            fi
            sudo yum install -y postgresql11-server &>> mylog.log
            if [ $? -ne 0 ]; then
                echo "PostgreSQL not installed! Check $HOME/mylog.log for more details."
                exit
            fi
            echo "Postgres11 Installed."
            break;;
            [Nn]* ) echo "User denied postgress installation. Skipping...."; break;;
            * ) echo "Please answer y/Y or n/N.";;
        esac
    done
else
    echo "pg_dump is already installed. Skipping."
fi

while true; do
    read -p "Install Oracle Instant Clients (Required for ora2pg tool)? (Y/N): " yn
    case $yn in
        [Yy]* ) 
        echo "Installing Oracle Instant Clients"
        wget https://download.oracle.com/otn_software/linux/instantclient/215000/oracle-instantclient-basic-21.5.0.0.0-1.x86_64.rpm &>> mylog.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check $HOME/mylog.log for more details."
            exit
        fi
        sudo yum install -y oracle-instantclient-basic-21.5.0.0.0-1.x86_64.rpm &>> mylog.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check $HOME/mylog.log for more details."
            exit
        fi
        rm oracle-instantclient-basic-21.5.0.0.0-1.x86_64.rpm

        wget https://download.oracle.com/otn_software/linux/instantclient/215000/oracle-instantclient-devel-21.5.0.0.0-1.x86_64.rpm &>> mylog.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check $HOME/mylog.log for more details."
            exit
        fi
        sudo yum install -y oracle-instantclient-devel-21.5.0.0.0-1.x86_64.rpm &>> mylog.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check $HOME/mylog.log for more details."
            exit
        fi
        rm oracle-instantclient-devel-21.5.0.0.0-1.x86_64.rpm

        wget https://download.oracle.com/otn_software/linux/instantclient/215000/oracle-instantclient-jdbc-21.5.0.0.0-1.x86_64.rpm &>> mylog.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check $HOME/mylog.log for more details."
            exit
        fi
        sudo yum install -y oracle-instantclient-jdbc-21.5.0.0.0-1.x86_64.rpm &>> mylog.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check $HOME/mylog.log for more details."
            exit
        fi
        rm oracle-instantclient-jdbc-21.5.0.0.0-1.x86_64.rpm

        wget https://download.oracle.com/otn_software/linux/instantclient/215000/oracle-instantclient-sqlplus-21.5.0.0.0-1.x86_64.rpm &>> mylog.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check $HOME/mylog.log for more details."
            exit
        fi
        sudo yum install -y oracle-instantclient-sqlplus-21.5.0.0.0-1.x86_64.rpm &>> mylog.log
        if [ $? -ne 0 ]; then
            echo "Instant Clients not installed! Check $HOME/mylog.log for more details."
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
            echo 'export ORACLE_HOME=/usr/lib/oracle/21/client64' >> ~/.bashrc
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
source ~/.bashrc

if [ -z ${LD_LIBRARY_PATH+x} ]; then
    while true; do
        read -p "LD_LIBRARY_PATH not set. Do you want to set it to $ORACLE_HOME/lib:$LD_LIBRARY_PATH? (Y/N): " yn
        case $yn in
            [Yy]* )
            echo 'export LD_LIBRARY_PATH=$ORACLE_HOME/lib:$LD_LIBRARY_PATH' >> ~/.bashrc
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
source ~/.bashrc

while true; do
    read -p "Do you want to include $ORACLE_HOME/bin to you PATH variable? (Y/N): " yn
    case $yn in
        [Yy]* )
        echo 'export PATH=$PATH:$ORACLE_HOME/bin' >> ~/.bashrc
        break;;
        [Nn]* )
        echo "Skipping...."
        break;;
        * ) echo "Please answer y/Y or n/N.";;
    esac
done
source ~/.bashrc

which ora2pg &>> temp.log
if  [ $? -ne 0 ]; then
    echo "ora2pg not found."
    while true; do
        read -p "Install ora2pg? (Y/N): " yn
        case $yn in
            [Yy]* )
            perl_version=$(perl --version | sed -n '2p' | cut -d "(" -f2 | cut -d ")" -f1 | cut -d "v" -f2)
            if [ -z $perl_version ]; then
                echo "perl not found."
                echo "Installing perl."
                sudo yum install -y perl &>> mylog.log
                if [ $? -ne 0 ]; then
                    echo "perl not installed! Check $HOME/mylog.log for more details."
                    exit
                fi
                echo "perl installed."
            elif [[ "$perl_version" < "5.10" ]]; then
                echo "perl Found, But atleast 5.10 version required."
                echo "Installing latest perl."
                sudo yum install -y perl &>> mylog.
                if [ $? -ne 0 ]; then
                    echo "perl not installed! Check $HOME/mylog.log for more details."
                    exit
                fi
                echo "perl installed."
            else
                echo "perl (5.10 or more) is already installed. Skipping."
            fi

            perl -e 'use DBI' &>> temp.log
            if [ $? -ne 0 ]; then
                echo "DBI not found."
                echo "Installing DBI."
                sudo yum install -y perl-DBI &>> mylog.log
                if [ $? -ne 0 ]; then
                    echo "DBI not installed! Check $HOME/mylog.log for more details."
                    exit
                fi
                echo "DBI installed."
            else
                dbi_version=$(perl -e 'use DBI; DBI->installed_versions();' | sed -n /DBI/p | cut -d ':' -f2 | cut -d ' ' -f2)
                if [[ "$dbi_version" < "1.614" ]]; then
                    echo "DBI Found, But atleast 1.614 version required."
                    echo "Installing latest DBI."
                    sudo yum install -y perl &>> mylog.log
                    if [ $? -ne 0 ]; then
                        echo "DBI not installed! Check $HOME/mylog.log for more details."
                        exit
                    fi
                    echo "DBI installed."
                else
                    echo "perl-DBI (1.614 or more) is already installed. Skipping."
                fi
            fi

            which cpanm &>> temp.log
            if [ $? -ne 0 ]; then
                echo "cpanm not found."
                echo "Installing cpanm."
                sudo yum install -y perl-App-cpanminus &>> mylog.log
                if [ $? -ne 0 ]; then
                    echo "cpanm not installed! Check $HOME/mylog.log for more details."
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
                sudo yum install -y mysql-devel &>> mylog.log
                if [ $? -ne 0 ]; then
                    echo "DBD::mysql not installed! Check $HOME/mylog.log for more details."
                    exit
                fi
                sudo cpanm DBD::mysql &>> mylog.log
                if [ $? -ne 0 ]; then
                    echo "DBD::mysql not installed! Check $HOME/mylog.log for more details."
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
                sudo cpanm Test::NoWarnings &>> mylog.log
                if [ $? -ne 0 ]; then
                    echo "DBD::Oracle not installed! Check $HOME/mylog.log for more details."
                    exit
                fi
                sudo cpanm DBD::Oracle &>> mylog.log
                if [ $? -ne 0 ]; then
                    echo "DBD::Oracle not installed! Check $HOME/mylog.log for more details."
                    exit
                fi
                echo "DBD::Oracle installed."
            else
                echo "DBD::Oracle is already installed. Skipping."
            fi

            wget https://github.com/darold/ora2pg/archive/refs/tags/v23.0.tar.gz &>> mylog.log
            if [ $? -ne 0 ]; then
                echo "ora2pg not installed! Check $HOME/mylog.log for more details."
                exit
            fi
            tar -xvf v23.0.tar.gz &>> mylog.log
            cd ora2pg-23.0/
            perl Makefile.PL &>> mylog.log
            if [ $? -ne 0 ]; then
                echo "ora2pg not installed! Check $HOME/mylog.log for more details."
                exit
            fi
            make &>> mylog.log && sudo make install &>> mylog.log
            if [ $? -ne 0 ]; then
                echo "ora2pg not installed! Check $HOME/mylog.log for more details."
                exit
            fi
            cd ~
            rm v23.0.tar.gz
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

git clone https://github.com/yugabyte/ybm.git | tee -a mylog.log
if [ $? -ne 0 ]; then
    echo "Authenticate your GitHub and then try running git clone https://github.com/yugabyte/ybm.git"
    exit
fi

cd ~/ybm/yb_migrate
go install
if [ $? -ne 0 ]; then
    echo "yb-migrate build FAILED! Check $HOME/mylog.log for more details."
    exit
fi
cd ~
