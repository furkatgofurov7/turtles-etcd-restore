package webhook

var installsh = `
#!/bin/sh

set -x
CURL_LOG="-v"

# Function to download the system agent binary with retries
download_agent_binary() {
    RETRY_COUNT=10
    DOWNLOAD_SUCCESS=false

    while [ $RETRY_COUNT -gt 0 ] && [ "$DOWNLOAD_SUCCESS" = false ]; do
        echo "Download attempt $((4 - $RETRY_COUNT)): Rancher System Agent binary"
        curl --connect-timeout 60 --max-time 300 --write-out "%{http_code}\n" ${CURL_CAFLAG} -v -fL "${CATTLE_SERVER}/assets/rancher-system-agent-amd64" -o "${CATTLE_AGENT_BIN_PREFIX}/bin/rancher-system-agent"

        if [ $? -eq 0 ] && [ -s "${CATTLE_AGENT_BIN_PREFIX}/bin/rancher-system-agent" ]; then
            DOWNLOAD_SUCCESS=true
            echo "Rancher System Agent binary downloaded successfully."
        else
            echo "Failed to download Rancher System Agent binary. Retrying..."
            RETRY_COUNT=$((RETRY_COUNT - 1))
            sleep 10
        fi
    done

    if [ "$DOWNLOAD_SUCCESS" = false ]; then
        echo "Failed to download Rancher System Agent binary after multiple attempts. Exiting."
        exit 1
    fi
}

echo "Downloading cert"
CACERT=$(mktemp)
curl --connect-timeout 60 --max-time 60 --write-out "%{http_code}\n" --insecure ${CURL_LOG} -fL "${CATTLE_SERVER}/cacerts" -o ${CACERT}

# Set up CURL_CAFLAG
CURL_CAFLAG="--cacert ${CACERT}"

# Call the function to download the system agent binary
download_agent_binary

chmod +x "${CATTLE_AGENT_BIN_PREFIX}/bin/rancher-system-agent"

echo "systemd: Creating service file"
cat <<-EOF >"/etc/systemd/system/rancher-system-agent.service"
[Unit]
Description=Rancher System Agent
Documentation=https://www.rancher.com
Wants=network-online.target
After=network-online.target
[Install]
WantedBy=multi-user.target
[Service]
EnvironmentFile=-/etc/default/rancher-system-agent
EnvironmentFile=-/etc/sysconfig/rancher-system-agent
EnvironmentFile=-/etc/systemd/system/rancher-system-agent.env
Type=simple
Restart=always
RestartSec=5s
Environment=CATTLE_LOGLEVEL=debug
Environment=CATTLE_AGENT_CONFIG=/etc/rancher/agent/config.yaml
ExecStart=${CATTLE_AGENT_BIN_PREFIX}/bin/rancher-system-agent sentinel
EOF

FILE_SA_ENV="/etc/systemd/system/rancher-system-agent.env"
echo "Creating environment file ${FILE_SA_ENV}"
install -m 0600 /dev/null "${FILE_SA_ENV}"
for i in "HTTP_PROXY" "HTTPS_PROXY" "NO_PROXY"; do
    eval v=\"\$$i\"
    if [ -z "${v}" ]; then
    env | grep -E -i "^${i}" | tee -a ${FILE_SA_ENV} >/dev/null
    else
    echo "$i=$v" | tee -a ${FILE_SA_ENV} >/dev/null
    fi
done

systemctl daemon-reload >/dev/null
echo "Enabling rancher-system-agent.service"
systemctl enable rancher-system-agent
echo "Starting/restarting rancher-system-agent.service"
systemctl restart rancher-system-agent
rm -f ${CATTLE_AGENT_VAR_DIR}/interlock/restart-pending
`
