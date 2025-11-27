#!/usr/bin/env bash

shopt -s dotglob
[ "${DEBUG:-}" == 'true' ] && set -x

set_default ()
{
    minecraft_version="1.14.3"
    minecraft_core_mod=""
    minecraft_core_mod_version=""
    repositories=('http://files.gameap.ru/minecraft')

    server_ip=''
    server_port=''
    server_query_port=''
    server_rcon_port=''
    server_rcon_password=''
}

parse_options ()
{
    command=$1
    shift

    if [[ $command == '-h' ]]; then
        command=''
    fi

    for i in "$@"
    do
        case $i in
            -h|--help)
                show_help
                exit 0
            ;;
            --version=*)
                minecraft_version=${i#*=}
                shift
            ;;
            --core-mod=*)
                minecraft_core_mod="${i#*=}"
                shift
            ;;
            --core-mod-version=*)
                minecraft_core_mod_version="${i#*=}"
                shift
            ;;
            --ip=*|--host=*)
                server_ip="${i#*=}"
                shift
            ;;
            --port=*)
                server_port="${i#*=}"
                shift
            ;;
            --query-port=*)
                server_query_port="${i#*=}"
                shift
            ;;
            --rcon-port=*)
                server_rcon_port="${i#*=}"
                shift
            ;;
            --rcon-password=*)
                server_rcon_password="${i#*=}"
                shift
            ;;
        esac
    done
}

show_help ()
{
    echo
    echo 'Usage:	./mcrun.sh COMMAND [OPTIONS]'
    echo
    echo 'Options:'
    echo '      --version=              Minecraft version'
    echo '      --core-mod=             Minecraft Core Mod'
    echo '      --core-mod-version=     Minecraft Core Mod Version'
    echo '      --ip=                   Server ip'
    echo '      --port=                 Server port'
    echo '      --query-port=           Server query port'
    echo '      --rcon-port=            Server rcon port'
    echo '      --rcon-password=        Server rcon password'
    echo
    echo 'Commands:'
    echo '  run         Run server'
    echo '  list        List available Minecraft Server versions'
    echo '  install     Install dependencies (Java etc.)'
    echo
    echo 'Examples:'
    echo '      ./mcrun.sh run --version="1.14.3" --ip=0.0.0.0 --port=25565'
    echo '      ./mcrun.sh run --version="1.14.3"'
    echo '      ./mcrun.sh run --core-mod=forge --version="1.14.3"'
    echo
}

show_logo ()
{
    echo
    echo '
                  ###           ####
               #######################
               #######################
              #######````````````#######
            ######   ############   ######
          #####  ####             ##  ########
      ########  ###               ####  ########
     ######## ###                 ###### ########
      ###### ####     ################### ######
       ####  ####     ################### #####
       #### #####     ###           #####  ####
       #### #####     ###           #####  ####
       ####  ####     #########     ##### #####
      ###### ####     #########     ##### ######
     ######## ###       ######      #### ########
      ########  ##                 ###  ########
       ########   ###            ###   #####
            #####    ############   ######
              #######````````````#######
                #######################
                #######################
                 #####          ####'
    echo
    echo '            GameAP Minecraft Server Runner'
    echo
    echo '----------------------------------------------------'
    echo
}

get_available_versions ()
{
    oldIFS=$IFS
    IFS=$'\n'
    available_versions=($(curl "${repositories[0]}/available_versions" 2> /dev/null))
    IFS=$oldIFS

    if [ "$?" -ne "0" ]; then
        echo "Unable to get available versions" >> /dev/stderr
        echo "Repo: ${repositories[0]}" >> /dev/stderr
        echo "Please change repo or try again later" >> /dev/stderr
        exit 1
    fi
}

list_available_versions ()
{
    get_available_versions

    echo
    echo '------------------------------------------'
    echo 'MOD    |    Version    |    Mod Version'
    echo '------------------------------------------'
    for (( i=0; i<${#available_versions[@]}; i++ ))
    do
        line=(${available_versions[$i]})
        echo "${line[0]}    |    ${line[1]}    |    ${line[2]:--}"
    done
}

check_version ()
{
    found=0

    for (( i=0; i<${#available_versions[@]}; i++ ))
    do
        line=(${available_versions[$i]})
        mod_version_line=${line[2]:-}

        if [[ ${minecraft_core_mod:-'vanilla'} == ${line[0]} \
            && $minecraft_version == ${line[1]} ]]; then

            if [ -z ${mod_version_line:-} ]; then
                found=1
                break
            fi

            if [ -z ${minecraft_core_mod_version:-} ] && [ -n $mod_version_line ]; then
                minecraft_core_mod_version=$mod_version_line
                found=1
                break
            fi

            if [[ ${minecraft_core_mod_version:-} == $mod_version_line ]]; then
                found=1
                break
            fi

            break;
        fi
    done

    if [ $found -eq "0" ]; then
        echo
        echo "Current version not available" >> /dev/stderr
        echo "Core: ${minecraft_core_mod:-'Minecraft Vanilla'}" >> /dev/stderr
        echo "Minecraft Version: ${minecraft_version}" >> /dev/stderr
        echo "Core Version: ${minecraft_core_mod_version:--}" >> /dev/stderr
        exit 1;
    fi
}

java_setup ()
{
    echo
    echo "Java installation..."

    if [[ ! -f "jre-8u211-linux-x64.tar.gz" ]]; then
        if ! curl -O http://files.gameap.ru/java/jre-8u211-linux-x64.tar.gz; then
            echo "Unable to download java" >> /dev/stderr
            exit 1
        fi
    fi

    if [[ ! -s "/usr/lib/jvm" ]]; then
        if ! mkdir /usr/lib/jvm; then
            echo "Unable to make directory /usr/lib/jvm" >> /dev/stderr
            exit 1
        fi
    fi

    if ! tar -xzf jre-8u211-linux-x64.tar.gz -C /usr/lib/jvm/; then
        echo "Unable to unpack java" >> /dev/stderr
        exit 1
    fi

    if ! update-alternatives --install /usr/bin/java java /usr/lib/jvm/jre1.8.0_211/bin/java 1051; then
        echo "Unable to install java alternative"
        exit 1
    fi

    if [[ -f "jre-8u211-linux-x64.tar.gz" ]]; then
        rm jre-8u211-linux-x64.tar.gz
    fi
}

detect_java ()
{
    repeat=$1
    if command -v java > /dev/null; then
        echo "Java Detected..."
    else
        echo "Java not found..."

        if [[ $EUID -ne 0 ]]; then
           echo "Please run as root 'install' to try install java" >> /dev/stderr
           exit 1
        fi

        if [ -z $repeat ]; then
            java_setup
            detect_java 1
        else
            echo "Please setup java manually" >> /dev/stderr
            exit 1
        fi
    fi
}

configure_vars ()
{
    case ${minecraft_core_mod} in
        forge)
            version_delim='-'
            download_jar_postfix='-installer'
            exec_jar_postfix=''
        ;;
        craftbukkit)
            version_delim='-'
            download_jar_postfix=''
            exec_jar_postfix=''
        ;;
        *)
            version_delim='.'
            download_jar_postfix=''
            exec_jar_postfix=''
        ;;
    esac

    if [ "$minecraft_core_mod" = "vanilla" ]; then
        minecraft_core_mod=""
    fi

    if [ -n "${minecraft_core_mod_version:-}" ]; then
        mod_version_delim="${version_delim}"
    fi

    file_prefix=${minecraft_core_mod:-'minecraft_server'}

    case ${minecraft_core_mod:-'vanilla'} in
        vanilla)
            jar_file_download="${file_prefix}${version_delim:-.}${minecraft_version}.jar"
            exec_jar="${file_prefix}${version_delim:-.}${minecraft_version}${exec_jar_postfix}.jar"
        ;;
        *)
            jar_file_download="${file_prefix}${version_delim:-.}${minecraft_version}${mod_version_delim:-}${minecraft_core_mod_version}${download_jar_postfix}.jar"
            exec_jar="${file_prefix}${version_delim:-.}${minecraft_version}${mod_version_delim:-}${minecraft_core_mod_version}${exec_jar_postfix}.jar"
        ;;
    esac
}

set_properties ()
{
    if [[ ! -f server.properties ]]; then
        return
    fi

    if [[ -n "${server_ip:-}" ]]; then
        sed -i "s/server-ip.*$/server-ip=${server_ip}/" server.properties
    fi

    if [[ -n "${server_port:-}" ]]; then
        sed -i "s/server-port.*$/server-port=${server_port}/" server.properties
    fi

    # QUERY Settings

    if [[ -n "${server_query_port:-}" ]]; then
        sed -i "s/query\.port.*$/query\.port=${server_query_port}/" server.properties
        sed -i "s/enable-query.*$/enable-query=true/" server.properties
    fi

    # RCON Settings

    if [[ -n "${server_rcon_port:-}" ]]; then
        sed -i "s/rcon\.port.*$/rcon\.port=${server_rcon_port}/" server.properties
    fi

    if [[ -n "${server_rcon_password:-}" ]]; then
        sed -i "s/rcon\.password.*$/rcon\.password=${server_rcon_password}/" server.properties
    fi

    if [[ -n "${server_rcon_port:-}" ]] && [[ -n "${server_rcon_password:-}" ]]; then
        sed -i "s/enable-rcon.*$/enable-rcon=true/" server.properties
    fi
}

run ()
{
    if [ ! -f $jar_file_download ]; then
        echo "Minecraft Server not found..."
        echo "Downloading..."
        echo

        get_available_versions
        check_version
        configure_vars

        #
        # $jar_file_download can be changed in configure_vars functions
        #
        if [ ! -f $jar_file_download ]; then
            download_path="${repositories[0]}/${minecraft_core_mod:-}"
            download_link="${download_path%/}/${jar_file_download}"

            if ! curl -qLf --output $jar_file_download $download_link; then
                echo "Unable to download minecraft server" >> /dev/stderr
                echo "Download link: ${download_link}" >> /dev/stderr
                echo "Please change repo or try again later" >> /dev/stderr
                exit 1
            fi

            chmod +x $jar_file_download

            if [[ ${minecraft_core_mod:-} == 'forge' ]]; then
                if ! java -jar "$jar_file_download" --installServer; then
                    echo "Unable to install Forge" >> /dev/stderr
                    exit 1
                fi
            fi
        fi
    fi

    if [ ! -f "eula.txt" ]; then
        echo "eula=true" > eula.txt
    fi

    set_properties

    echo "Running Minecraft Server..."
    echo

    case ${minecraft_core_mod:-'minecraft_server'} in
        minecraft_server|forge|craftbukkit|cauldron)
            java -jar $exec_jar
        ;;

        spigot)
            java -XX:MaxPermSize=1G -XX:+UseConcMarkSweepGC -jar $exec_jar
        ;;

        *)
            java -jar $exec_jar
        ;;
    esac
}

main ()
{
    show_logo
    configure_vars

    case ${command:-} in
        run|start)
            detect_java
            run
        ;;

        list)
            echo
            echo "Available Minecraft Server versions"
            echo

            list_available_versions
        ;;

        install)
            detect_java
        ;;

        *)
            show_help
        ;;
    esac
}

set_default
parse_options "$@"
main
