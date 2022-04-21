#!/bin/bash
set -e

#########################
# Repo specific content #
#########################

VERIFY_CHECKSUM=${VERIFY_CHECKSUM:-'0'}
ALIAS_NAME=${ALIAS_NAME:-''}
OWNER=${OWNER:-'cnrancher'}
REPO=${REPO:-'autok3s'}
SUCCESS_CMD="$REPO version"
BINLOCATION=${BINLOCATION:-'/usr/local/bin'}
KUBEEXPLORER_REPO=${KUBEEXPLORER_REPO:-'kube-explorer'}
KUBEEXPLORER_DOWNLOAD_URL=https://github.com/$OWNER/$KUBEEXPLORER_REPO/releases/download
KUBEEXPLORER_VERSION=v0.2.8

#   - INSTALL_AUTOK3S_MIRROR
#     For Chinese users, set INSTALL_AUTOK3S_MIRROR=cn to use the mirror address to accelerate
#     autok3s binary file download, and the default mirror address is rancher-mirror.rancher.cn

if [ "${INSTALL_AUTOK3S_MIRROR}" = cn ]; then
    AUTOK3S_DOWNLOAD_URL=https://rancher-mirror.rancher.cn/$REPO
    version=$(curl -sS $AUTOK3S_DOWNLOAD_URL/channels/latest)
    KUBEEXPLORER_DOWNLOAD_URL=https://rancher-mirror.rancher.cn/$KUBEEXPLORER_REPO

else
    AUTOK3S_DOWNLOAD_URL=https://github.com/$OWNER/$REPO/releases/download
    version=$(curl -sI https://github.com/$OWNER/$REPO/releases/latest | grep -i "location:" | awk -F"/" '{ printf "%s", $NF }' | tr -d '\r')
fi

###############################
# Content common across repos #
###############################

if [ ! $version ]; then
    echo "Failed while attempting to install $REPO. Please manually install:"
    echo ""
    echo "1. Open your web browser and go to https://github.com/$OWNER/$REPO/releases"
    echo "2. Download the latest release for your platform. Call it '$REPO'."
    echo "3. chmod +x ./$REPO"
    echo "4. mv ./$REPO $BINLOCATION"
    if [ -n "$ALIAS_NAME" ]; then
        echo "5. ln -sf $BINLOCATION/$REPO /usr/local/bin/$ALIAS_NAME"
    fi
    exit 1
fi

hasCli() {

    hasCurl=$(which curl)
    if [ "$?" = "1" ]; then
        echo "You need curl to use this script."
        exit 1
    fi
}

checkHash(){

    sha_cmd="sha256sum"

    if [ ! -x "$(command -v $sha_cmd)" ]; then
    sha_cmd="shasum -a 256"
    fi

    if [ -x "$(command -v $sha_cmd)" ]; then

    targetFileDir=${targetFile%/*}

    sha_file_url=$AUTOK3S_DOWNLOAD_URL/$version/sha256sum.txt
    (cd $targetFileDir && curl -sSL $sha_file_url | grep $REPO$suffix |$sha_cmd -c >/dev/null)

        if [ "$?" != "0" ]; then
            rm $targetFile
            echo "Binary checksum didn't match. Exiting"
            exit 1
        fi
    fi
}

# --- set arch and suffix, fatal if architecture not supported ---
setup_verify_arch() {
    if [ -z "$ARCH" ]; then
        ARCH=$(uname -m)
    fi
    case $ARCH in
        amd64)
            ARCH=amd64
            ;;
        x86_64)
            ARCH=amd64
            ;;
        arm64)
            ARCH=arm64
            ;;
        aarch64)
            ARCH=arm64
            ;;
        arm*)
            ARCH=arm
            ;;
        *)
            fatal "Unsupported architecture $ARCH"
    esac
}

getPackage() {
    uname=$(uname)
    userid=$(id -u)

    setup_verify_arch

    case $uname in
    "Darwin")
        suffix="_darwin_$ARCH"
        ;;
    "MINGW"*)
        suffix="_windows_$ARCH.exe"
        BINLOCATION="$HOME/bin"
        mkdir -p $BINLOCATION
    ;;
    "Linux")
        suffix="_linux_$ARCH"
    ;;
    esac

    targetFile="/tmp/$REPO$suffix"

    if [ "$userid" != "0" ]; then
        targetFile="$(pwd)/$REPO$suffix"
    fi

    if [ -e "$targetFile" ]; then
        rm "$targetFile"
    fi

    url=$AUTOK3S_DOWNLOAD_URL/$version/$REPO$suffix
    echo "Downloading package $url as $targetFile"

    curl -sSL $url --output "$targetFile"

    if [ "$?" = "0" ]; then

        if [ "$VERIFY_CHECKSUM" = "1" ]; then
            checkHash
        fi

    chmod +x "$targetFile"

    echo "Download complete."

    if [ ! -w "$BINLOCATION" ]; then

            echo
            echo "============================================================"
            echo "  The script was run as a user who is unable to write"
            echo "  to $BINLOCATION. To complete the installation the"
            echo "  following commands may need to be run manually."
            echo "============================================================"
            echo
            echo "  sudo cp $REPO$suffix $BINLOCATION/$REPO"

            if [ -n "$ALIAS_NAME" ]; then
                echo "  sudo ln -sf $BINLOCATION/$REPO $BINLOCATION/$ALIAS_NAME"
            fi

            echo

        else

            echo
            echo "Running with sufficient permissions to attempt to move $REPO to $BINLOCATION"

            if [ ! -w "$BINLOCATION/$REPO" ] && [ -f "$BINLOCATION/$REPO" ]; then

            echo
            echo "================================================================"
            echo "  $BINLOCATION/$REPO already exists and is not writeable"
            echo "  by the current user.  Please adjust the binary ownership"
            echo "  or run sh/bash with sudo."
            echo "================================================================"
            echo
            exit 1

            fi

            mv $targetFile $BINLOCATION/$REPO

            if [ "$?" = "0" ]; then
                echo "New version of $REPO installed to $BINLOCATION"
            fi

            if [ -e "$targetFile" ]; then
                rm "$targetFile"
            fi

            if [ -n "$ALIAS_NAME" ]; then
                if [ ! -L $BINLOCATION/$ALIAS_NAME ]; then
                    ln -s $BINLOCATION/$REPO $BINLOCATION/$ALIAS_NAME
                    echo "Creating alias '$ALIAS_NAME' for '$REPO'."
                fi
            fi

            ${SUCCESS_CMD}
        fi
    fi
}

getKubeExplorer() {
    uname=$(uname)
    userid=$(id -u)

    setup_verify_arch

    case $uname in
    "Darwin")
        suffix="-darwin-$ARCH"
        ;;
    "MINGW"*)
        suffix="-windows-$ARCH.exe"
        BINLOCATION="$HOME/bin"
        mkdir -p $BINLOCATION
    ;;
    "Linux")
        suffix="-linux-$ARCH"
    ;;
    esac

    targetFile="/tmp/$KUBEEXPLORER_REPO$suffix"

    if [ "$userid" != "0" ]; then
        targetFile="$(pwd)/$KUBEEXPLORER_REPO$suffix"
    fi

    if [ -e "$targetFile" ]; then
        rm "$targetFile"
    fi

    url=$KUBEEXPLORER_DOWNLOAD_URL/$KUBEEXPLORER_VERSION/$KUBEEXPLORER_REPO$suffix
    echo "Downloading package $url as $targetFile"

    curl -sSL $url --output "$targetFile"

    chmod +x "$targetFile"

    echo "Download complete."

    if [ ! -w "$BINLOCATION" ]; then

        echo
        echo "============================================================"
        echo "  The script was run as a user who is unable to write"
        echo "  to $BINLOCATION. To complete the installation the"
        echo "  following commands may need to be run manually."
        echo "============================================================"
        echo
        echo "  sudo cp $KUBEEXPLORER_REPO$suffix $BINLOCATION/$KUBEEXPLORER_REPO"
        echo

    else

        echo
        echo "Running with sufficient permissions to attempt to move $KUBEEXPLORER_REPO to $BINLOCATION"

        if [ ! -w "$BINLOCATION/$KUBEEXPLORER_REPO" ] && [ -f "$BINLOCATION/$KUBEEXPLORER_REPO" ]; then

            echo
            echo "================================================================"
            echo "  $BINLOCATION/$KUBEEXPLORER_REPO already exists and is not writeable"
            echo "  by the current user.  Please adjust the binary ownership"
            echo "  or run sh/bash with sudo."
            echo "================================================================"
            echo
            exit 1

        fi

        mv $targetFile $BINLOCATION/$KUBEEXPLORER_REPO

        if [ "$?" = "0" ]; then
            echo "New version of $KUBEEXPLORER_REPO installed to $BINLOCATION"
        fi

        if [ -e "$targetFile" ]; then
            rm "$targetFile"
        fi

    fi
}

create_symlinks() {
    if [ ! -e ${BINLOCATION}/kubectl ]; then
        which_cmd=$(command -v kubectl 2>/dev/null || true)
        if [ -z "${which_cmd}" ]; then
            echo "Creating ${BINLOCATION}/kubectl symlink to autok3s"
            $SUDO ln -sf autok3s ${BINLOCATION}/kubectl
        else
            echo "Skipping ${BINLOCATION}/kubectl symlink to autok3s, command exists in PATH at ${which_cmd}"
        fi
    else
        echo "Skipping ${BINLOCATION}/kubectl symlink to autok3s, already exists"
    fi
}

# --- create kill process script ---
create_killprocess() {
    echo "Creating kill script ${BINLOCATION}/autok3s-killall.sh"
    $SUDO tee ${BINLOCATION}/autok3s-killall.sh >/dev/null << \EOF
#!/bin/sh
[ $(id -u) -eq 0 ] || exec sudo $0 $@
killtree() {
    kill -9 $(
        { set +x; } 2>/dev/null;
        for pid in $@; do
            echo $pid
        done
        set -x;
    ) 2>/dev/null
}

pstree() {
    ps -e -o pid= -o args= | sed -e 's/^ *//; s/\s\s*/\t/;' | grep -E "kube-explorer|autok3s" | cut -f1
}

killtree $({ set +x; } 2>/dev/null; pstree; set -x)
EOF
    $SUDO chmod 755 ${BINLOCATION}/autok3s-killall.sh
    $SUDO chown root:root ${BINLOCATION}/autok3s-killall.sh
}

# --- create uninstall script ---
create_uninstall() {
    echo "Creating uninstall script ${BINLOCATION}/autok3s-uninstall.sh"
    $SUDO tee ${BINLOCATION}/autok3s-uninstall.sh >/dev/null << EOF
#!/bin/sh
set -x
[ \$(id -u) -eq 0 ] || exec sudo \$0 \$@

if [ -L ${BINLOCATION}/kubectl ]; then
    rm -f ${BINLOCATION}/kubectl
fi

remove_uninstall() {
    rm -f ${BINLOCATION}/autok3s-uninstall.sh
    rm -f ${BINLOCATION}/autok3s-killall.sh
}
trap remove_uninstall EXIT

${BINLOCATION}/autok3s-killall.sh

rm -f ${BINLOCATION}/autok3s
rm -f ${BINLOCATION}/kube-explorer

EOF
    $SUDO chmod 755 ${BINLOCATION}/autok3s-uninstall.sh
    $SUDO chown root:root ${BINLOCATION}/autok3s-uninstall.sh
}

hasCli
getPackage
getKubeExplorer
create_symlinks
create_killprocess
create_uninstall
