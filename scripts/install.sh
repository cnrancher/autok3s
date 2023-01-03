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
KUBEEXPLORER_VERSION=v0.3.0
SUDO=sudo
if [ $(id -u) -eq 0 ]; then
    SUDO=
fi

#   - INSTALL_AUTOK3S_MIRROR
#     For Chinese users, set INSTALL_AUTOK3S_MIRROR=cn to use the mirror address to accelerate
#     autok3s binary file download, and the default mirror address is rancher-mirror.oss-cn-beijing.aliyuncs.com

if [ "${INSTALL_AUTOK3S_MIRROR}" = cn ]; then
    AUTOK3S_DOWNLOAD_URL=https://rancher-mirror.oss-cn-beijing.aliyuncs.com/$REPO
    version=$(curl -sS $AUTOK3S_DOWNLOAD_URL/channels/latest)
    KUBEEXPLORER_DOWNLOAD_URL=https://rancher-mirror.oss-cn-beijing.aliyuncs.com/$KUBEEXPLORER_REPO

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

    if [ "$VERIFY_CHECKSUM" = "1" ]; then
        checkHash
    fi

    echo "Download complete."

    $SUDO mv -f $targetFile $BINLOCATION/$REPO
    $SUDO chmod +x $BINLOCATION/$REPO

    if [ -e "$targetFile" ]; then
        rm "$targetFile"
    fi

    if [ -n "$ALIAS_NAME" ]; then
        if [ ! -L $BINLOCATION/$ALIAS_NAME ]; then
            $SUDO ln -s $BINLOCATION/$REPO $BINLOCATION/$ALIAS_NAME
            echo "Creating alias '$ALIAS_NAME' for '$REPO'."
        fi
    fi

    ${SUCCESS_CMD}
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
    echo "Download complete."

    $SUDO mv $targetFile $BINLOCATION/$KUBEEXPLORER_REPO
    $SUDO chmod +x $BINLOCATION/$KUBEEXPLORER_REPO

    if [ -e "$targetFile" ]; then
        rm "$targetFile"
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
}
trap remove_uninstall EXIT

pids=\$(ps -e -o pid= -o args= | sed -e 's/^ *//; s/\s\s*/\t/;' | grep -E "kube-explorer|autok3s " | grep -v grep | cut -f1)
set +m
for pid in \$pids; do
        kill -9 \$pid 2>&1
done
set -m

rm -f ${BINLOCATION}/autok3s
rm -f ${BINLOCATION}/kube-explorer

EOF
    $SUDO chmod 755 ${BINLOCATION}/autok3s-uninstall.sh
}

hasCli
getPackage
getKubeExplorer
create_symlinks
create_uninstall
