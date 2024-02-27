# turtles-etcd-restore

## Setting up the environment

To set up the environment, navigate to the root of the repository and run the `make dev-env` command.

```bash
    make dev-env SSH_KEY="yourkeyname"
```

The `Makefile` target `dev-env` sets up the environment by executing the `scripts/etcd-backup-restore-dev.sh` 
script with the `SSH_KEY` argument. Under the hood, it performs the following steps:

1. Creates a kind cluster.
2. Deploys cert-manager, rancher, CAPI Operator with Rancher Turtles.
3. Deploys RKE2 provider (`examples/rke2-provider.yaml`).
4. Deploys AWS provider (`examples/aws-provider.yaml`).
5. Deploys RKE2 Cluster in AWS (`examples/rke2-aws-cluster.yaml`).

To clean up the environment, run the following command from the root of the repo:

```bash
    make clean-dev-env
```