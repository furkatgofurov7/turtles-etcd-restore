# turtles-etcd-restore

## Setting up the environment

To set up the environment, navigate to the root of the repository and run:

```bash
    make dev-env SSH_KEY="yourkeyname"
```

The `Makefile` target sets up the environment by executing the `scripts/etcd-backup-restore-dev.sh` 
script with the `SSH_KEY` argument. Under the hood, it performs the following steps:

1. Creates a kind cluster.
2. Deploys cert-manager, rancher, CAPI Operator with Rancher Turtles.
3. Deploys RKE2 provider (`examples/rke2-provider.yaml`).
4. Deploys AWS provider (`examples/aws-provider.yaml`).
5. Deploys RKE2 Cluster in AWS (`examples/rke2-aws-cluster.yaml`).

To have a fully working setup, it is a pre-requisite for system agent installation
to configure a rancher server URL. To configure it, we need to access the Rancher UI by
opening 2 separate new terminals:

1. In the first terminal run:

```bash
    kubectl port-forward --namespace cattle-system svc/rancher 10000:443
```

2. In the second terminal:

```bash
    ngrok http https://localhost:10000
```

Follow the auto-generated link from second terminal to access the UI.

## Performing the backup and restore

When all machine in the cluster are ready, ETCDMachineBackup object should appear on the management cluster soon.

```bash
    kubectl get etcdmachinebackup -A
```

To perform a restore run the following command:

```bash
    CLUSTER_NAMESPACE=your-cluster-namespace
    CLUSTER_NAME=your-cluster-name
    ETCD_MACHINE_BACKUP_NAME=your-etcd-machine-backup-name
    kubectl apply -f examples/etcd-restore.yaml
```

## Cleanup

To clean up the environment, run the following command from the root of the repo:

```bash
    make clean-dev-env
```
