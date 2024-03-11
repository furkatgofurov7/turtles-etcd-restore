package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/furkatgofurov7/turtles-etcd-restore/api/v1alpha1"
)

type Plan struct {
	OneTimeInstructions []OneTimeInstruction `json:"instructions,omitempty"`
}

type OneTimeInstruction struct {
	Name       string   `json:"name,omitempty"`
	Image      string   `json:"image,omitempty"`
	Env        []string `json:"env,omitempty"`
	Args       []string `json:"args,omitempty"`
	Command    string   `json:"command,omitempty"`
	SaveOutput bool     `json:"saveOutput,omitempty"`
}

func killAllInstruction() OneTimeInstruction {
	return OneTimeInstruction{
		Name:    "shutdown",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			"if [ -z $(command -v rke2) ] && [ -z $(command -v rke2-killall.sh) ]; then echo rke2 does not appear to be installed; exit 0; else rke2-killall.sh; fi",
		},
		SaveOutput: true,
	}
}

func etcdRestoreInstruction(snapshot *v1alpha1.EtcdMachineBackup) OneTimeInstruction {
	return OneTimeInstruction{
		Name:    "etcd-restore",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			"rke2 server --cluster-reset",
			fmt.Sprintf("--cluster-reset-restore-path=%s", strings.TrimPrefix(snapshot.Spec.Location, "file://")),
		},
		SaveOutput: true,
	}
}

// https://github.com/rancher/rancher/issues/41174
func manifestRemovalInstruction() OneTimeInstruction {
	return OneTimeInstruction{
		Name:    "remove-server-manifests",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			"rm -rf /var/lib/rancher/rke2/server/manifests/rke2-*.yaml",
		},
		SaveOutput: true,
	}
}

func removeEtcdDataInstruction() OneTimeInstruction {
	return OneTimeInstruction{
		Name:    "remove-etcd-db-dir",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			"rm -rf /var/lib/rancher/rke2/server/db/etcd",
		},
		SaveOutput: true,
	}
}

func startRKE2Instruction() OneTimeInstruction {
	return OneTimeInstruction{
		Name:    "start-rke2",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			"systemctl start rke2-server.service",
		},
		SaveOutput: true,
	}
}

func instructionsAsJson(instructions []OneTimeInstruction) ([]byte, error) {
	return json.Marshal(Plan{OneTimeInstructions: instructions})
}

func isPlanApplied(plan, appliedChecksum []byte) bool {
	result := sha256.Sum256(plan)
	planHash := hex.EncodeToString(result[:])

	return planHash == string(appliedChecksum)
}
