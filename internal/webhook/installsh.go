package webhook

var installsh = `
#!/bin/sh

set -x
CURL_LOG="-v"

echo "Downloading cert"
CACERT=$(mktemp)
curl --connect-timeout 60 --max-time 60 --write-out "%{http_code}\n" --insecure ${CURL_LOG} -fL "${CATTLE_SERVER}/cacerts" -o ${CACERT}

echo "Download system agent binary"
CURL_CAFLAG="--cacert ${CACERT}"
CATTLE_AGENT_BIN_PREFIX="/usr/local"
mkdir -p ${CATTLE_AGENT_BIN_PREFIX}/bin
curl --connect-timeout 60 --max-time 300 --write-out "%{http_code}\n" ${CURL_CAFLAG} -v -fL "${CATTLE_SERVER}/assets/rancher-system-agent-amd64" -o "${CATTLE_AGENT_BIN_PREFIX}/bin/rancher-system-agent"
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
